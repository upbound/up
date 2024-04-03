// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package profile

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FromKubeconfig finds the profile by a given user kubeconfig. It returns
// a related profile, the current group/namespace, and the controlplane if the
// kubeconfig points to a controlplane through mxe-router.
func FromKubeconfig(ctx context.Context, profiles map[string]Profile, conf *clientcmdapi.Config) (string, *Profile, types.NamespacedName, error) {
	return findProfileByKubeconfig(ctx, profiles, conf, getIngressHost)
}

func findProfileByKubeconfig(ctx context.Context, profiles map[string]Profile, conf *clientcmdapi.Config, getIngressHost func(ctx context.Context, cfg *rest.Config) (string, error)) (string, *Profile, types.NamespacedName, error) { // nolint:gocyclo // TODO: shorten
	// get base URL of Upbound profiles
	profileURLs := make(map[string]string)
	for name, p := range profiles {
		if !p.IsSpace() {
			continue
		}
		confCtx, ok := conf.Contexts[p.KubeContext]
		if !ok {
			continue
		}
		cluster, ok := conf.Clusters[confCtx.Cluster]
		if !ok {
			continue
		}
		profileURLs[name] = cluster.Server
	}

	// get user cluster URL
	if conf.CurrentContext == "" {
		return "", nil, types.NamespacedName{}, fmt.Errorf("no current context in kubeconfig")
	}
	confCtx, ok := conf.Contexts[conf.CurrentContext]
	if !ok {
		return "", nil, types.NamespacedName{}, fmt.Errorf("current context %q not found in kubeconfig", conf.CurrentContext)
	}
	cluster, ok := conf.Clusters[confCtx.Cluster]
	if !ok {
		return "", nil, types.NamespacedName{}, fmt.Errorf("cluster %q not found in kubeconfig", confCtx.Cluster)
	}

	// find profile by URL
	for name, profileURL := range profileURLs {
		if strings.TrimSuffix(profileURL, "/") == strings.TrimSuffix(cluster.Server, "/") {
			p := profiles[name]
			ns := confCtx.Namespace
			if ns == "" {
				ns = corev1.NamespaceDefault
			}
			return name, &p, types.NamespacedName{Namespace: ns}, nil
		}
		url, err := url.Parse(profileURL)
		if err != nil {
			continue
		}

		// profile points to Spaces API?
		if strings.HasPrefix(cluster.Server, strings.TrimSuffix(url.String(), "/")+"/") {
			p := profiles[name]

			ctp, found := ParseSpacesK8sURL(strings.TrimSuffix(cluster.Server, "/"))
			if !found {
				return "", nil, types.NamespacedName{}, fmt.Errorf("not connected to a control plane or Space")
			}

			return name, &p, ctp, nil
		}
	}

	// still not found. Try ingresses.
	for name, p := range profiles {
		if !p.IsSpace() {
			continue
		}
		pconf := conf.DeepCopy()
		pconf.CurrentContext = p.KubeContext
		cfg, err := clientcmd.NewDefaultClientConfig(*pconf, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			continue
		}
		ingressHost, err := getIngressHost(ctx, cfg)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(ingressHost, "https://") {
			ingressHost = "https://" + ingressHost
		}
		if strings.HasPrefix(cluster.Server, strings.TrimSuffix(ingressHost, "/")+"/") {
			ctp, found := ParseSpacesK8sURL(strings.TrimSuffix(cluster.Server, "/"))
			if !found {
				return "", nil, types.NamespacedName{}, fmt.Errorf("not connected to a control plane or Space")
			}
			return name, &p, ctp, nil
		}
	}

	return "", nil, types.NamespacedName{}, nil
}

func getIngressHost(ctx context.Context, cfg *rest.Config) (string, error) {
	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		return "", err
	}
	mxpConfig := &corev1.ConfigMap{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "mxp-config", Namespace: "upbound-system"}, mxpConfig); err != nil {
		return "", err
	}
	ingressHost := mxpConfig.Data["ingress.host"]
	return ingressHost, nil
}
