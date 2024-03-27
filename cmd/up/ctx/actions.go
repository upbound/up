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
	// new context
	groupKubeconfig, err := g.space.kubeconfig.RawConfig()
	if err != nil {
		return "", err
	}
	p, err := upCtx.Cfg.GetUpboundProfile(g.space.profile)
	if err != nil {
		return "", err
	}
	groupKubeconfig.CurrentContext = p.KubeContext
	if err := clientcmdapi.MinifyConfig(&groupKubeconfig); err != nil {
		return "", err
	}
	groupKubeconfig.Contexts[groupKubeconfig.CurrentContext].Namespace = g.name

	prevContext, err := mergeIntoKubeConfig(&groupKubeconfig, kubeContext, kubeContext+upboundPreviousContextSuffix, "", kube.VerifyKubeConfig(upCtx.WrapTransport))
	if err != nil {
		return "", err
	}
	if err := writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}

	return fmt.Sprintf(contextSwitchedFmt, kubeContext, g.Breadcrumbs()), nil
}

// Accept upserts a controlplane context to the current kubeconfig.
func (ctp *ControlPlane) Accept(ctx context.Context, upCtx *upbound.Context, kubeContext string) (msg string, err error) {
	groupCfg, err := ctp.space.kubeconfig.ClientConfig()
	if err != nil {
		return "", err
	}
	groupKubeconfig, err := ctp.space.kubeconfig.RawConfig()
	if err != nil {
		return "", err
	}
	p, err := upCtx.Cfg.GetUpboundProfile(ctp.space.profile)
	if err != nil {
		return "", err
	}
	groupKubeconfig.CurrentContext = p.KubeContext

	// construct a context pointing to the controlplane, either via ingress or
	// connection secret as fallback
	ingress, ca, err := profile.GetIngressHost(ctx, groupCfg)
	if err != nil {
		return "", err
	}
	if ingress == "" {
		return "", fmt.Errorf("ingress condiguration not found")

	}

	// via ingress, with same credentials, but different URL
	ctpConfig := groupKubeconfig.DeepCopy()
	if err := clientcmdapi.MinifyConfig(ctpConfig); err != nil {
		return "", err
	}
	ctpCtx := ctpConfig.Contexts[ctpConfig.CurrentContext]
	ctpConfig.Clusters[ctpCtx.Cluster].Server = fmt.Sprintf("https://%s/apis/spaces.upbound.io/v1beta1/namespaces/%s/controlplanes/%s/k8s", ingress, ctp.Namespace, ctp.Name)
	ctpConfig.Clusters[ctpCtx.Cluster].CertificateAuthorityData = ca
	ctpConfig.Contexts[ctpConfig.CurrentContext].Namespace = "default"

	// merge into kubeconfig
	prevContext, err := mergeIntoKubeConfig(ctpConfig, kubeContext, kubeContext+upboundPreviousContextSuffix, "", kube.VerifyKubeConfig(upCtx.WrapTransport))
	if err != nil {
		return "", err
	}
	if err := writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}

	return fmt.Sprintf(contextSwitchedFmt, kubeContext, ctp.Breadcrumbs()), nil
}

// mergeIntoKubeConfig merges the current context of the passed kubeconfig into
// default kubeconfig on disk, health checks and writes it back. The previous
// context for "up ctx -" is returned.
func mergeIntoKubeConfig(newConf *clientcmdapi.Config, newContext, previousContext string, existingFilePath string, preCheck ...func(cfg *clientcmdapi.Config) error) (oldContext string, err error) { // nolint:gocyclo // TODO: shorten
	po := clientcmd.NewDefaultPathOptions()
	po.LoadingRules.ExplicitPath = existingFilePath
	conf, err := po.GetStartingConfig()
	if err != nil {
		return "", err
	}

	// normalize broken configs
	if _, ok := conf.Contexts[conf.CurrentContext]; !ok {
		conf.Contexts[conf.CurrentContext] = &clientcmdapi.Context{}
	}

	// store current upbound cluster, authInfo, context as upbound-previous if named "upbound"
	oldContext = conf.CurrentContext
	oldCluster := conf.Contexts[conf.CurrentContext].Cluster
	newCluster := newConf.Contexts[newConf.CurrentContext].Cluster
	if oldCluster == newCluster && !reflect.DeepEqual(newConf.Clusters[oldCluster], conf.Clusters[newCluster]) {
		conf.Clusters[previousContext] = conf.Clusters[oldCluster]
		conf.Contexts[oldContext].Cluster = previousContext
	}
	oldAuthInfo := conf.Contexts[conf.CurrentContext].AuthInfo
	newAuthInfo := newConf.Contexts[newConf.CurrentContext].AuthInfo
	if oldAuthInfo == newAuthInfo && !reflect.DeepEqual(newConf.AuthInfos[newAuthInfo], conf.AuthInfos[oldAuthInfo]) {
		conf.AuthInfos[previousContext] = conf.AuthInfos[oldAuthInfo]
		conf.Contexts[oldContext].AuthInfo = previousContext
	}
	if oldContext == newContext && !reflect.DeepEqual(newConf.Contexts[oldContext], conf.Contexts[newContext]) {
		conf.Contexts[previousContext] = conf.Contexts[oldContext]
		oldContext = previousContext
	}

	// merge in new context
	conf.Clusters[newCluster] = newConf.Clusters[newCluster]
	conf.AuthInfos[newAuthInfo] = newConf.AuthInfos[newAuthInfo]
	conf.Contexts[newContext] = newConf.Contexts[newConf.CurrentContext]
	conf.CurrentContext = newContext

	// health check
	for _, check := range preCheck {
		withDefContext := *conf
		withDefContext.CurrentContext = newConf.CurrentContext
		if err := check(&withDefContext); err != nil {
			return "", err
		}
	}

	return oldContext, clientcmd.ModifyConfig(po, *conf, true)
}
