package caching

import (
	// "bytes"
	"context"
	// "hash"
	// "hash/fnv"
	"log"
	// "net"
	// "strings"
	// "time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Controller !
type Controller struct {
	coreV1 corev1client.CoreV1Interface
}

// NewController !
func NewController(kubernetes *kubernetes.Clientset) *Controller {

	log.Println("Caching: constructing controller")

	return &Controller{
		coreV1: kubernetes.CoreV1(),
	}
}

// BIG TODO: cache these things for like 60 seconds at a time or whatever

// GetConfigMap fetches a ConfigMap.
func (ctl *Controller) GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {

	log.Println("Caching: fetching ConfigMap", namespace, name)

	configMap, err := ctl.coreV1.ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})

	return configMap, err
}

// GetSecret fetches a secret.
func (ctl *Controller) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {

	log.Println("Caching: fetching secret", namespace, name)

	secret, err := ctl.coreV1.Secrets(namespace).Get(ctx, name, metav1.GetOptions{})

	return secret, err
}
