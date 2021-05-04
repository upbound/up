package kube

import (
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	// KubeconfigDir is the default kubeconfig directory.
	KubeconfigDir = ".kube"
	// KubeconfigFile is the default kubeconfig file.
	KubeconfigFile = "config"
)

// GetKubeClient constructs a Kubernetes client from the specified kubeconfig.
func GetKubeClient(path string) (*kubernetes.Clientset, error) {
	if path == "" {
		path = filepath.Join(homedir.HomeDir(), KubeconfigDir, KubeconfigFile)
	}
	config, err := clientcmd.BuildConfigFromFlags("", path)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
