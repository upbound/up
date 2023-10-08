package controlplane

import (
	"fmt"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestConnect(t *testing.T) {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).RawConfig()

	fmt.Println(cfg.CurrentContext)
	fmt.Println(err)

	// check if current context is for a control plane
	//
}

func SwitchContext(kubeConfig *api.Config, otherContext string) error {
	kubeConfig.CurrentContext = otherContext
	return clientcmd.ModifyConfig(clientcmd.NewDefaultClientConfigLoadingRules(), *kubeConfig, false)
}
