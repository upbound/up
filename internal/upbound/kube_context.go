package upbound

import (
	"context"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/upbound/up/internal/profile"
	"k8s.io/apimachinery/pkg/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildCurrentContextClient creates a K8s client using the current Kubeconfig
// defaulting to the current Kubecontext
func (c *Context) BuildCurrentContextClient() (client.Client, error) {
	rest, err := c.Kubecfg.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get kube config")
	}

	sc, err := client.New(rest, client.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "error creating kube client")
	}
	return sc, nil
}

// IsSelfHostedSpaceContext returns true if the current context is pointed at a
// self-hosted space cluster
func (c *Context) IsSelfHostedSpaceContext(ctx context.Context) (bool, error) {
	client, err := c.BuildCurrentContextClient()
	if err != nil {
		return false, err
	}

	host, _, err := profile.GetIngressHost(ctx, client)
	return host != "", err
}

func (c *Context) getCurrentContext() (context *clientcmdapi.Context, cluster *clientcmdapi.Cluster, exists bool) {
	// todo: Add support for overriding current context as part of CLI args

	config, err := c.Kubecfg.RawConfig()
	if err != nil {
		return nil, nil, false
	}

	current := config.CurrentContext
	if current == "" {
		return nil, nil, false
	}

	context, exists = config.Contexts[current]
	if !exists {
		return nil, nil, false
	}

	cluster, exists = config.Clusters[context.Cluster]
	return context, cluster, exists
}

func (c *Context) ParseCurrentSpaceContextURL() (types.NamespacedName, bool) {
	_, cluster, exists := c.getCurrentContext()
	if !exists {
		return types.NamespacedName{}, false
	}

	return profile.ParseSpacesK8sURL(strings.TrimSuffix(cluster.Server, "/"))
}
