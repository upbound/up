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

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"

	"github.com/pkg/errors"
	"github.com/upbound/up/internal/kube"
	upbound "github.com/upbound/up/internal/upbound"
)

const (
	upboundPreviousContextSuffix = "-previous"
)

// Accept upserts the "upbound" kubeconfig context and cluster to the current
// kubeconfig, pointing to the space.
func (s *Space) Accept(ctx context.Context, upCtx *upbound.Context, kubeContext string) (msg string, err error) {
	spaceContext, err := writeContext(ctx, upCtx, *s, types.NamespacedName{}, kubeContext)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, spaceContext, s.Breadcrumbs()), nil
}

// Accept upserts the "upbound" kubeconfig context and cluster to the current
// kubeconfig, pointing to the group.
func (g *Group) Accept(ctx context.Context, upCtx *upbound.Context, kubeContext string) (msg string, err error) {
	groupContext, err := writeContext(ctx, upCtx, g.space, types.NamespacedName{Namespace: g.name}, kubeContext)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, groupContext, g.Breadcrumbs()), nil
}

// Accept upserts a controlplane context and cluster to the current kubeconfig.
func (ctp *ControlPlane) Accept(ctx context.Context, upCtx *upbound.Context, kubeContext string) (msg string, err error) {
	ctpContext, err := writeContext(ctx, upCtx, ctp.group.space, ctp.NamespacedName(), kubeContext)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, ctpContext, ctp.Breadcrumbs()), nil
}

// writeContext upserts the current kubeconfig with the upserted upbound context
// and cluster to point to the given space and resource
func writeContext(ctx context.Context, upCtx *upbound.Context, space Space, ctp types.NamespacedName, kubecontext string) (newContext string, err error) { //nolint:gocyclo
	prev, err := upCtx.Kubecfg.RawConfig()
	if err != nil {
		return "", errors.Wrap(err, "unable to get kube config")
	}

	srcConf, err := buildSpacesClient(space.ingress, space.ca, space.authInfo, ctp).RawConfig()
	if err != nil {
		return "", err
	}

	// assumes the current context
	ctpConf, prevContext, err := mergeUpboundContext(&prev, &srcConf, srcConf.CurrentContext, kubecontext)
	if err != nil {
		return "", err
	}
	if contextDeepEqual(ctpConf, prevContext, ctpConf.CurrentContext) {
		return prevContext, nil
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

	return ctpConf.CurrentContext, nil
}

// mergeUpboundContext copies the provided group context into the config under
// the provided context name, updates the current context to the new context and
// renaming the previous current context.
// Note: We add all of the information to the `*-previous` context in this
// method because when we call `activateContext`, it gets swapped with the
// correct context name.
func mergeUpboundContext(dest, src *clientcmdapi.Config, srcContext, destContext string) (ctpConf *clientcmdapi.Config, prevContext string, err error) { // nolint:gocyclo // little long, but well tested
	dest = dest.DeepCopy()

	if _, ok := src.Contexts[srcContext]; !ok {
		return nil, "", fmt.Errorf("context %q not found in kubeconfig", srcContext)
	}
	groupCluster, ok := src.Clusters[src.Contexts[srcContext].Cluster]
	if !ok {
		return nil, "", fmt.Errorf("cluster %q not found in kubeconfig", src.Contexts[srcContext].Cluster)
	}
	authInfo := src.AuthInfos[src.Contexts[srcContext].AuthInfo]

	if _, ok := dest.Clusters[destContext+upboundPreviousContextSuffix]; ok {
		// make room for upbound-previous cluster
		freeCluster := destContext + upboundPreviousContextSuffix
		for d := 1; true; d++ {
			s := fmt.Sprintf("%s%d", freeCluster, d)
			if _, ok := dest.Clusters[s]; !ok {
				freeCluster = s
				break
			}
		}
		renamed := 0
		for name, ctx := range dest.Contexts {
			if ctx.Cluster == destContext+upboundPreviousContextSuffix && name != destContext+upboundPreviousContextSuffix {
				ctx.Cluster = freeCluster
				renamed++
			}
		}
		if renamed > 0 {
			dest.Clusters[freeCluster] = dest.Clusters[destContext+upboundPreviousContextSuffix]
		}
	}

	dest.Clusters[destContext+upboundPreviousContextSuffix] = ptr.To(*groupCluster)

	if dest.CurrentContext == destContext+upboundPreviousContextSuffix {
		// make room for upbound-previous context
		dest.Contexts[destContext] = dest.Contexts[destContext+upboundPreviousContextSuffix]
		dest.CurrentContext = destContext
	}
	dest.Contexts[destContext+upboundPreviousContextSuffix] = ptr.To(*src.Contexts[srcContext])
	dest.Contexts[destContext+upboundPreviousContextSuffix].Cluster = destContext + upboundPreviousContextSuffix

	if authInfo != nil {
		dest.AuthInfos[destContext+upboundPreviousContextSuffix] = ptr.To(*authInfo)
		dest.Contexts[destContext+upboundPreviousContextSuffix].AuthInfo = destContext + upboundPreviousContextSuffix
	}

	return activateContext(dest, destContext+upboundPreviousContextSuffix, destContext)
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
