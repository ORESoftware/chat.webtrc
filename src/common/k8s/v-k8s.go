// source file path: ./src/common/k8s/v-k8s.go
package vk8s

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

func ReadSecretFromKubernetes(namespace, secretName string) (map[string][]byte, error) {
	// Get the Kubernetes config
	config, err := rest.InClusterConfig()
	if err != nil {
		// If running outside of Kubernetes, fall back to using the kubeconfig file
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.ExpandEnv("$HOME/.kube/config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Get the secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret.Data, nil
}

func init() {
	// Specify the namespace and secret name
	namespace := "your-namespace"
	secretName := "your-secret-name"

	secretData, err := ReadSecretFromKubernetes(namespace, secretName)
	if err != nil {
		panic(err.Error())
	}

	// Print the secret data
	fmt.Println("Secret Data:")
	for key, value := range secretData {
		fmt.Printf("%s: %s\n", key, string(value))
	}
}
