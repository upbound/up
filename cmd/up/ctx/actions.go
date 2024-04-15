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

package ctx

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/profile"
	upbound "github.com/upbound/up/internal/upbound"
)

const (
	upboundPreviousContextSuffix = "-previous"
)

// Accept upserts the "upbound" kubeconfig context to the current kubeconfig,
// pointing to the group.
func (g *Group) Accept(ctx context.Context, upCtx *upbound.Context, kubeContext string) (msg string, err error) {
	// find existing space context
	p, err := upCtx.Cfg.GetUpboundProfile(g.space.profile)
	if err != nil {
		return "", err
	}
	loader, err := p.GetSpaceKubeConfig()
	if err != nil {
		return "", err
	}
	conf, err := loader.RawConfig()
	if err != nil {
		return "", err
	}

	// switch to group context
	groupConf, prevContext, err := g.accept(&conf, p, kubeContext)
	if err != nil {
		return "", err
	}
	if contextDeepEqual(groupConf, prevContext, groupConf.CurrentContext) {
		return fmt.Sprintf(contextSwitchedFmt, groupConf.CurrentContext, g.Breadcrumbs()), nil
	}
	if err := kube.VerifyKubeConfig(upCtx.WrapTransport)(groupConf); err != nil {
		return "", err
	}

	// write back
	if err := clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), *groupConf, true); err != nil {
		return "", err
	}
	if err := writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}

	return fmt.Sprintf(contextSwitchedFmt, groupConf.CurrentContext, g.Breadcrumbs()), nil
}

func (g *Group) accept(conf *clientcmdapi.Config, p profile.Profile, kubeContext string) (groupConf *clientcmdapi.Config, prevContext string, err error) {
	conf = conf.DeepCopy()

	groupContext := p.KubeContext
	if groupContext == "" {
		groupContext = conf.CurrentContext
	}
	if _, ok := conf.Contexts[groupContext]; !ok {
		return nil, "", fmt.Errorf("context %q not found in kubeconfig", groupContext)
	}

	// create upbound-previous context because the group differs from the profile
	if conf.Contexts[groupContext].Namespace != g.name {
		// make room for upbound-previous context
		if conf.CurrentContext == kubeContext+upboundPreviousContextSuffix {
			conf.Contexts[kubeContext] = conf.Contexts[kubeContext+upboundPreviousContextSuffix]
			conf.CurrentContext = kubeContext
		}

		conf.Contexts[kubeContext+upboundPreviousContextSuffix] = ptr.To(*conf.Contexts[groupContext])
		conf.Contexts[kubeContext+upboundPreviousContextSuffix].Namespace = g.name
		groupContext = kubeContext + upboundPreviousContextSuffix
	}
	conf, prevContext, err = activateContext(conf, groupContext, kubeContext)
	if err != nil {
		return nil, "", err
	}

	return conf, prevContext, nil
}

// Accept upserts a controlplane context to the current kubeconfig.
func (ctp *ControlPlane) Accept(ctx context.Context, upCtx *upbound.Context, preferredKubeContext string) (msg string, err error) { // nolint:gocyclo // little long, but well tested
	// find existing space context
	p, err := upCtx.Cfg.GetUpboundProfile(ctp.space.profile)
	if err != nil {
		return "", err
	}
	loader, err := p.GetSpaceKubeConfig()
	if err != nil {
		return "", err
	}
	groupCfg, err := loader.ClientConfig()
	if err != nil {
		return "", err
	}
	conf, err := loader.RawConfig()
	if err != nil {
		return "", err
	}

	// construct a context pointing to the controlplane via ingress, same
	// credentials, but different URL
	ingress, ca, err := profile.GetIngressHost(ctx, groupCfg)
	if err != nil {
		return "", err
	}
	if ingress == "" {
		return "", fmt.Errorf("ingress condiguration not found")
	}
	ctpConf, prevContext, err := ctp.accept(&conf, p, ingress, ca, preferredKubeContext)
	if err != nil {
		return "", err
	}
	if contextDeepEqual(ctpConf, prevContext, ctpConf.CurrentContext) {
		return fmt.Sprintf(contextSwitchedFmt, prevContext, ctp.Breadcrumbs()), nil
	}
	if err := kube.VerifyKubeConfig(upCtx.WrapTransport)(ctpConf); err != nil {
		return "", err
	}

	// write back
	if err := clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), *ctpConf, true); err != nil {
		return "", err
	}
	if err := writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}

	return fmt.Sprintf(contextSwitchedFmt, ctpConf.CurrentContext, ctp.Breadcrumbs()), nil
}

func (ctp *ControlPlane) accept(conf *clientcmdapi.Config, p profile.Profile, ingress string, ca []byte, kubeContext string) (ctpConf *clientcmdapi.Config, prevContext string, err error) { // nolint:gocyclo // little long, but well tested
	conf = conf.DeepCopy()

	groupContext := p.KubeContext
	if groupContext == "" {
		groupContext = conf.CurrentContext
	}
	if _, ok := conf.Contexts[groupContext]; !ok {
		return nil, "", fmt.Errorf("context %q not found in kubeconfig", groupContext)
	}
	groupCluster, ok := conf.Clusters[conf.Contexts[groupContext].Cluster]
	if !ok {
		return nil, "", fmt.Errorf("cluster %q not found in kubeconfig", conf.Contexts[groupContext].Cluster)
	}

	if _, ok := conf.Clusters[kubeContext+upboundPreviousContextSuffix]; ok {
		// make room for upbound-previous cluster
		freeCluster := kubeContext + upboundPreviousContextSuffix
		for d := 1; true; d++ {
			s := fmt.Sprintf("%s%d", freeCluster, d)
			if _, ok := conf.Clusters[s]; !ok {
				freeCluster = s
				break
			}
		}
		renamed := 0
		for name, ctx := range conf.Contexts {
			if ctx.Cluster == kubeContext+upboundPreviousContextSuffix && name != kubeContext+upboundPreviousContextSuffix {
				ctx.Cluster = freeCluster
				renamed++
			}
		}
		if renamed > 0 {
			conf.Clusters[freeCluster] = conf.Clusters[kubeContext+upboundPreviousContextSuffix]
		}
	}
	conf.Clusters[kubeContext+upboundPreviousContextSuffix] = ptr.To(*groupCluster)
	conf.Clusters[kubeContext+upboundPreviousContextSuffix].Server = fmt.Sprintf("https://%s/apis/spaces.upbound.io/v1beta1/namespaces/%s/controlplanes/%s/k8s", ingress, ctp.Namespace, ctp.Name)
	conf.Clusters[kubeContext+upboundPreviousContextSuffix].CertificateAuthorityData = ca

	if conf.CurrentContext == kubeContext+upboundPreviousContextSuffix {
		// make room for upbound-previous context
		conf.Contexts[kubeContext] = conf.Contexts[kubeContext+upboundPreviousContextSuffix]
		conf.CurrentContext = kubeContext
	}
	conf.Contexts[kubeContext+upboundPreviousContextSuffix] = ptr.To(*conf.Contexts[groupContext])
	conf.Contexts[kubeContext+upboundPreviousContextSuffix].Cluster = kubeContext + upboundPreviousContextSuffix
	conf.Contexts[kubeContext+upboundPreviousContextSuffix].Namespace = "default"

	return activateContext(conf, kubeContext+upboundPreviousContextSuffix, kubeContext)
}

func contextDeepEqual(conf *clientcmdapi.Config, a, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	prev := conf.Contexts[a]
	current := conf.Contexts[b]
	if prev == nil && current == nil {
		return true
	}
	if prev == nil || current == nil {
		return false
	}
	if !reflect.DeepEqual(conf.Clusters[prev.Cluster], conf.Clusters[current.Cluster]) {
		return false
	}
	if !reflect.DeepEqual(conf.AuthInfos[prev.AuthInfo], conf.AuthInfos[current.AuthInfo]) {
		return false
	}

	return false
}
