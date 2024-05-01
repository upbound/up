package upbound

import (
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/upbound/up/internal/profile"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HasValidContext returns true if the kube configuration attached to the
// context is valid and usable.
func (c *Context) HasValidContext() bool {
	// todo(redbackthomson): Add support for overriding current context as part
	// of CLI args
	config, err := c.Kubecfg.RawConfig()
	if err != nil {
		return false
	}

	return clientcmd.ConfirmUsable(config, "") == nil
}

// BuildCurrentContextClient creates a K8s client using the current Kubeconfig
// defaulting to the current Kubecontext
func (c *Context) BuildCurrentContextClient() (client.Client, error) {
	rest, err := c.Kubecfg.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get kube config")
	}

	// todo(redbackthomson): Delete once spaces-api is able to accept protobuf
	// requests
	rest.ContentConfig.ContentType = "application/json"

	sc, err := client.New(rest, client.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "error creating kube client")
	}
	return sc, nil
}

func (c *Context) GetCurrentContext() (context *clientcmdapi.Context, cluster *clientcmdapi.Cluster, auth *clientcmdapi.AuthInfo, exists bool) {
	// todo: Add support for overriding current context as part of CLI args

	config, err := c.Kubecfg.RawConfig()
	if err != nil {
		return nil, nil, nil, false
	}

	current := config.CurrentContext
	if current == "" {
		return nil, nil, nil, false
	}

	context, exists = config.Contexts[current]
	if !exists {
		return nil, nil, nil, false
	}

	cluster, exists = config.Clusters[context.Cluster]

	if context.AuthInfo == "" {
		return context, cluster, nil, exists
	}

	auth, exists = config.AuthInfos[context.AuthInfo]
	return context, cluster, auth, exists
}

func (c *Context) ParseCurrentSpaceContextURL() (string, types.NamespacedName, bool) {
	_, cluster, _, exists := c.GetCurrentContext()
	if !exists {
		return "", types.NamespacedName{}, false
	}

	return profile.ParseSpacesK8sURL(strings.TrimSuffix(cluster.Server, "/"))
}
