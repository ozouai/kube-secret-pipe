package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"sync"

	"gopkg.in/yaml.v2"
	"gopkg.zouai.io/colossus/clog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	ctx := context.Background()
	configFile := flag.String("config-file", "", "The config file")
	flag.Parse()
	configData, err := ioutil.ReadFile(*configFile)
	if err != nil {
		clog.Errf(ctx, err, "Error reading config file")
		panic("")
	}

	baseConfig := &Config{}
	err = yaml.Unmarshal(configData, &baseConfig)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// kubeConfig, err := clientcmd.BuildConfigFromFlags("", "/tmp/kubeconfig")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	client, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(client)
	if err != nil {
		panic(err)
	}
	for _, c := range baseConfig.Targets {
		compiler := &Compiler{
			Clientset: clientset,
			Config:    c,
		}
		err = compiler.Start(context.Background())
		if err != nil {
			panic(err)
		}
	}
	CoreWG.Wait()

}

var CoreWG sync.WaitGroup

func watchSecret(ctx context.Context, clientset *kubernetes.Clientset, namespace string, secret string, resultChan chan<- *v1.Secret) error {
	watcher, err := clientset.CoreV1().Secrets(namespace).Watch(ctx, metav1.SingleObject(metav1.ObjectMeta{
		Name:      secret,
		Namespace: namespace,
	}))
	if err != nil {
		return fmt.Errorf("error watching secret: %w", err)
	}
	CoreWG.Add(1)
	go func() {
		defer CoreWG.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case event, isOpen := <-watcher.ResultChan():
				if !isOpen {
					return
				}
				switch event.Type {
				case watch.Added, watch.Modified:
					resultChan <- event.Object.(*v1.Secret)
				}
			}
		}
	}()
	return nil
}

func watchConfigMap(ctx context.Context, clientset *kubernetes.Clientset, namespace string, configmap string) (<-chan *v1.ConfigMap, error) {
	watcher, err := clientset.CoreV1().ConfigMaps(namespace).Watch(ctx, metav1.SingleObject(metav1.ObjectMeta{
		Name: configmap,
	}))
	if err != nil {
		return nil, fmt.Errorf("error watching secret: %w", err)
	}
	resultChan := make(chan *v1.ConfigMap)
	CoreWG.Add(1)
	go func() {
		defer CoreWG.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case event, isOpen := <-watcher.ResultChan():
				if !isOpen {
					return
				}
				switch event.Type {
				case watch.Added, watch.Modified:
					resultChan <- event.Object.(*v1.ConfigMap)
				}
			}
		}
	}()
	return resultChan, nil
}
