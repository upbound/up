package kube

import (
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	// KubeconfigDir is the default kubeconfig directory.
	KubeconfigDir = ".kube"
	// KubeconfigFile is the default kubeconfig file.
	KubeconfigFile = "config"
)

// GetKubeConfig constructs a Kubernetes REST config from the specified kubeconfig.
func GetKubeConfig(path string) (*rest.Config, error) {
	if path == "" {
		path = filepath.Join(homedir.HomeDir(), KubeconfigDir, KubeconfigFile)
	}
	return clientcmd.BuildConfigFromFlags("", path)
}
