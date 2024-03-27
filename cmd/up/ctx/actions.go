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

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/cmd/up/controlplane/kubeconfig"
	"github.com/upbound/up/internal/controlplane/space"
	"github.com/upbound/up/internal/kube"
	upbound "github.com/upbound/up/internal/upbound"
)

const (
	upboundContext         = "upbound"
	upboundPreviousContext = "upbound-previous"
)

// Accept upserts the "upbound" kubeconfig context to the current kubeconfig,
// pointing to the group.
func (g *Group) Accept(ctx context.Context, upCtx *upbound.Context) (msg string, err error) {
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

	prevContext, err := mergeIntoKubeConfig(&groupKubeconfig, upboundContext, "", kube.VerifyKubeConfig(upCtx.WrapTransport))
	if err != nil {
		return "", err
	}
	if err := writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}

	return fmt.Sprintf(contextSwitchedFmt, g.Breadcrumbs()), nil
}

// Accept upserts a controlplane context to the current kubeconfig.
func (ctp *ControlPlane) Accept(ctx context.Context, upCtx *upbound.Context) (msg string, err error) {
	// get connection secret
	config, err := ctp.space.kubeconfig.ClientConfig()
	if err != nil {
		return "", err
	}
	cl, err := dynamic.NewForConfig(config)
	if err != nil {
		return "", err
	}
	getter := space.New(cl)
	ctpConfig, err := getter.GetKubeConfig(ctx, ctp.NamespacedName)
	if err != nil {
		return "", err
	}

	// merge in new context
	ctpConfig, err = kubeconfig.ExtractControlPlaneContext(ctpConfig, kubeconfig.ExpectedConnectionSecretContext(upCtx.Account, ctp.Name), upboundContext)
	if err != nil {
		return "", err
	}
	prevContext, err := mergeIntoKubeConfig(ctpConfig, upboundContext, "", kube.VerifyKubeConfig(upCtx.WrapTransport))
	if err != nil {
		return "", err
	}
	if err := writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}

	return fmt.Sprintf(contextSwitchedFmt, ctp.Breadcrumbs()), nil
}

// mergeIntoKubeConfig merges the current context of the passed kubeconfig into
// default kubeconfig on disk, health checks and writes it back. The previous
// context for "up ctx -" is returned.
func mergeIntoKubeConfig(newConf *clientcmdapi.Config, newContext string, existingFilePath string, preCheck ...func(cfg *clientcmdapi.Config) error) (oldContext string, err error) { // nolint:gocyclo // TODO: shorten
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
	if oldCluster == newCluster && newConf.Clusters[oldCluster] != conf.Clusters[newCluster] {
		conf.Clusters[upboundPreviousContext] = conf.Clusters[oldCluster]
		conf.Contexts[oldContext].Cluster = upboundPreviousContext
	}
	oldAuthInfo := conf.Contexts[conf.CurrentContext].AuthInfo
	newAuthInfo := newConf.Contexts[newConf.CurrentContext].AuthInfo
	if oldAuthInfo == newAuthInfo && newConf.AuthInfos[newAuthInfo] != conf.AuthInfos[oldAuthInfo] {
		conf.AuthInfos[upboundPreviousContext] = conf.AuthInfos[oldAuthInfo]
		conf.Contexts[oldContext].AuthInfo = upboundPreviousContext
	}
	if oldContext == newContext && newConf.Contexts[oldContext] != conf.Contexts[newContext] {
		conf.Contexts[upboundPreviousContext] = conf.Contexts[oldContext]
		oldContext = upboundPreviousContext
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
