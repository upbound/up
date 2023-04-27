// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	// KubeconfigDir is the default kubeconfig directory.
	KubeconfigDir = ".kube"
	// KubeconfigFile is the default kubeconfig file.
	KubeconfigFile = "config"
	// UpboundKubeconfigKeyFmt is the format for Upbound control plane entries
	// in a kubeconfig file.
	UpboundKubeconfigKeyFmt = "upbound-%s"

	// UpboundK8sResource is appended to the end of the kubeconfig server path.
	UpboundK8sResource = "k8s"
)

// GetKubeConfig constructs a Kubernetes REST config from the specified
// kubeconfig, or falls back to same defaults as kubectl.
func GetKubeConfig(path string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = path
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
}

// BuildControlPlaneKubeconfig builds a kubeconfig entry for a control plane.
func BuildControlPlaneKubeconfig(proxy *url.URL, id string, token string) *api.Config { //nolint:interfacer
	conf := api.NewConfig()
	key := fmt.Sprintf(UpboundKubeconfigKeyFmt, strings.ReplaceAll(id, "/", "-"))
	proxy.Path = path.Join(proxy.Path, id, UpboundK8sResource)
	conf.Clusters[key] = &api.Cluster{
		Server: proxy.String(),
	}
	conf.AuthInfos[key] = &api.AuthInfo{
		Token: token,
	}
	conf.Contexts[key] = &api.Context{
		Cluster:  key,
		AuthInfo: key,
	}
	conf.CurrentContext = key
	return conf
}

// ApplyControlPlaneKubeconfig applies a control plane kubeconfig to an existing
// kubeconfig file and sets it as the current context.
func ApplyControlPlaneKubeconfig(mcpConf *api.Config, existingFilePath string) error {
	po := clientcmd.NewDefaultPathOptions()
	po.LoadingRules.ExplicitPath = existingFilePath
	conf, err := po.GetStartingConfig()
	if err != nil {
		return err
	}
	for k, v := range mcpConf.Clusters {
		conf.Clusters[k] = v
	}
	for k, v := range mcpConf.AuthInfos {
		conf.AuthInfos[k] = v
	}
	for k, v := range mcpConf.Contexts {
		conf.Contexts[k] = v
	}
	conf.CurrentContext = mcpConf.CurrentContext

	// In the case of user error, for example providing an invalid access token,
	// we do not want to set it as the current context as it will be invalid.
	// A client allows us to verify connectivity in addition to a well-formed config.
	clientConfig := clientcmd.NewDefaultClientConfig(*conf, &clientcmd.ConfigOverrides{})

	// A rest.Config is required for clients.
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		// For example, an invalid token was passed.
		return err
	}

	// We could use any client for this check, but discovery allows us to perform
	// additional validation if so desired. For now we perform a lightweight operation.
	if _, err := clientset.DiscoveryClient.ServerVersion(); err != nil {
		// For example, the target cluster does not exist.
		return err
	}

	return clientcmd.ModifyConfig(po, *conf, true)
}
