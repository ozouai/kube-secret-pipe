package kubesecretpipe

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func Spawn(ctx context.Context, wg *sync.WaitGroup, clientset *kubernetes.Clientset, config *Config) error {
	for _, c := range config.Targets {
		compiler := &Compiler{
			Clientset: clientset,
			Config:    c,
			wg:        wg,
		}
		err := compiler.Start(ctx)
		if err != nil {
			return fmt.Errorf("error starting compiler for '%s/%s': %w", c.TargetNamespace, c.TargetName, err)
		}
	}
	return nil
}

func watchSecret(ctx context.Context, wg *sync.WaitGroup, clientset *kubernetes.Clientset, namespace string, secret string, resultChan chan<- *v1.Secret) error {
	watcher, err := clientset.CoreV1().Secrets(namespace).Watch(ctx, metav1.SingleObject(metav1.ObjectMeta{
		Name:      secret,
		Namespace: namespace,
	}))
	if err != nil {
		return fmt.Errorf("error watching secret: %w", err)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
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

func watchConfigMap(ctx context.Context, wg *sync.WaitGroup, clientset *kubernetes.Clientset, namespace string, configmap string) (<-chan *v1.ConfigMap, error) {
	watcher, err := clientset.CoreV1().ConfigMaps(namespace).Watch(ctx, metav1.SingleObject(metav1.ObjectMeta{
		Name: configmap,
	}))
	if err != nil {
		return nil, fmt.Errorf("error watching secret: %w", err)
	}
	resultChan := make(chan *v1.ConfigMap)
	wg.Add(1)
	go func() {
		defer wg.Done()
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
