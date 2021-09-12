package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"html/template"

	"gopkg.zouai.io/colossus/clog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Compiler struct {
	Config         *ConfigTarget
	Clientset      *kubernetes.Clientset
	inputConfigMap *corev1.ConfigMap
	inputSecrets   map[string]*corev1.Secret
	outputHash     []byte
}

func (m *Compiler) Start(ctx context.Context) error {
	clog.Infof(ctx, "Starting Compiler for '%s/%s'", m.Config.TargetNamespace, m.Config.TargetName)
	m.inputSecrets = map[string]*corev1.Secret{}
	existingSecret, err := m.Clientset.CoreV1().Secrets(m.Config.TargetNamespace).Get(ctx, m.Config.TargetName, metav1.GetOptions{})
	if err != nil {
		clog.Err(ctx, err, "Got an error attempting to get the output secret")
		existingSecret = nil
	}
	m.outputHash = secretToHash(existingSecret)

	inputConfigMap, err := m.Clientset.CoreV1().ConfigMaps(m.Config.TargetNamespace).Get(ctx, m.Config.TargetName, metav1.GetOptions{})
	if err != nil {
		clog.Errf(ctx, err, "Got an error getting the input configmap")
		return fmt.Errorf("error getting input configmap: %w", err)
	}
	m.inputConfigMap = inputConfigMap
	for _, secretConfig := range m.Config.Secrets {
		secret, err := m.Clientset.CoreV1().Secrets(secretConfig.Namespace).Get(ctx, secretConfig.Name, metav1.GetOptions{})
		if err != nil {
			clog.Errf(ctx, err, "Got an error getting one of the input secrets '%s/%s'", secretConfig.Namespace, secretConfig.Name)
			return fmt.Errorf("error getting secretConfig: %w", err)
		}
		m.inputSecrets[secretConfig.Namespace+"/"+secretConfig.Name] = secret
	}

	firstOutput, err := m.compileFromCache(ctx)
	if err != nil {
		clog.Errf(ctx, err, "Error compiling first version")
		return fmt.Errorf("error compiling configmap: %w", err)
	}
	firstHash := secretToHash(firstOutput)
	if existingSecret == nil {
		clog.Info(ctx, "Uploading new version")
		_, err = m.Clientset.CoreV1().Secrets(m.Config.TargetNamespace).Create(ctx, firstOutput, metav1.CreateOptions{})
		if err != nil {
			clog.Errf(ctx, err, "Error uploading new version")
		}
	} else if bytes.Compare(firstHash, m.outputHash) != 0 {
		clog.Info(ctx, "Uploading new version")
		_, err = m.Clientset.CoreV1().Secrets(m.Config.TargetNamespace).Update(ctx, firstOutput, metav1.UpdateOptions{})
		if err != nil {
			clog.Errf(ctx, err, "Error uploading new version")
		}
	}
	inputConfigUpdate, err := watchConfigMap(ctx, m.Clientset, m.inputConfigMap.Namespace, m.inputConfigMap.Name)
	if err != nil {
		clog.Errf(ctx, err, "Error watching configmap for updates")
		return fmt.Errorf("error watching configmap for updates: %w", err)
	}
	secretUpdateChannel := make(chan *corev1.Secret)
	for _, secretConfig := range m.inputSecrets {
		err := watchSecret(ctx, m.Clientset, secretConfig.Namespace, secretConfig.Name, secretUpdateChannel)
		if err != nil {
			clog.Errf(ctx, err, "Error watching secret for updates")
			return fmt.Errorf("error watching secret for updates: %w", err)
		}
	}
	CoreWG.Add(1)
	go func() {
		defer CoreWG.Done()
		for {
			select {
			case cf := <-inputConfigUpdate:
				m.inputConfigMap = cf
				m.update(ctx)
			case s := <-secretUpdateChannel:
				m.inputSecrets[s.Namespace+"/"+s.Name] = s
				m.update(ctx)
			}
		}
	}()
	return nil
}

func (m *Compiler) update(ctx context.Context) error {
	output, err := m.compileFromCache(ctx)
	if err != nil {
		clog.Errf(ctx, err, "Error compiling first version")
		return fmt.Errorf("error compiling configmap: %w", err)
	}
	outputHash := secretToHash(output)
	if bytes.Compare(outputHash, m.outputHash) != 0 {
		clog.Info(ctx, "Uploading new version")
		_, err = m.Clientset.CoreV1().Secrets(m.Config.TargetNamespace).Update(ctx, output, metav1.UpdateOptions{})
		if err != nil {
			clog.Errf(ctx, err, "Error uploading new version")
		}
	}
	return nil
}

func (m *Compiler) compileFromCache(ctx context.Context) (*corev1.Secret, error) {
	outputConfigMap := &corev1.Secret{}
	outputConfigMap.Name = m.Config.TargetName
	outputConfigMap.Namespace = m.Config.TargetNamespace
	outputConfigMap.Data = map[string][]byte{}
	for k, v := range m.inputConfigMap.BinaryData {
		outputConfigMap.Data[k] = v
	}
	templateData := m.getTemplateData(ctx)
	for k, v := range m.inputConfigMap.Data {
		tmpl, err := template.New(m.Config.TargetName).Parse(v)
		if err != nil {
			clog.Errf(ctx, err, "Error parsing key '%s' of input configmap '%s/%s' for template", k, m.inputConfigMap.Name, m.inputConfigMap.Namespace)
			continue
		}
		buf := bytes.NewBufferString("")
		err = tmpl.Execute(buf, templateData)
		if err != nil {
			clog.Errf(ctx, err, "Error executing template for key '%s' of input configmap '%s/%s'", k, m.inputConfigMap.Name, m.inputConfigMap.Namespace)
			continue
		}

		outputConfigMap.Data[k] = buf.Bytes()
	}
	return outputConfigMap, nil
}

func (m *Compiler) getTemplateData(ctx context.Context) map[string]interface{} {
	output := map[string]interface{}{}
	for secretKey, secretConfig := range m.Config.Secrets {
		secret := m.inputSecrets[secretConfig.Namespace+"/"+secretConfig.Name]
		obj := map[string]string{}
		output[secretKey] = obj
		for k, v := range secret.Data {
			obj[k] = string(v)
		}
	}
	return output
}

func configMapToHash(configMap *corev1.ConfigMap) []byte {
	hasher := sha256.New()
	if configMap != nil && configMap.Data != nil {
		for k, v := range configMap.Data {
			hasher.Write([]byte(k))
			hasher.Write([]byte(v))
		}
	}
	return hasher.Sum([]byte{})
}

func secretToHash(secret *corev1.Secret) []byte {
	hasher := sha256.New()
	if secret != nil && secret.Data != nil {
		for k, v := range secret.Data {
			hasher.Write([]byte(k))
			hasher.Write([]byte(v))
		}
	}
	return hasher.Sum([]byte{})
}
