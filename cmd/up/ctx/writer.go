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
	"errors"
	"fmt"
	"io/fs"
	"reflect"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"

	"github.com/upbound/up/internal/upbound"
)

type kubeContextWriter interface {
	Write(config *clientcmdapi.Config) error
}

type printWriter struct {
}

var _ kubeContextWriter = &printWriter{}

// Write implements kubeContextWriter.Write.
func (p *printWriter) Write(config *clientcmdapi.Config) error {
	b, err := clientcmd.Write(*config)
	if err != nil {
		return err
	}

	fmt.Print(string(b))
	return nil
}

type fileWriter struct {
	upCtx        *upbound.Context
	fileOverride string
	kubeContext  string

	writeLastContext func(string) error
	verify           func(cfg *clientcmdapi.Config) error
	modifyConfig     func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error
}

var _ kubeContextWriter = &fileWriter{}

// Write implements kubeContextWriter.Write.
func (f *fileWriter) Write(config *clientcmdapi.Config) error {
	outConfig, err := f.loadOutputKubeconfig()
	if err != nil {
		return err
	}

	ctpConf, prevContext, err := f.upsertContext(config, outConfig)
	if err != nil {
		return err
	}

	pathOptions := clientcmd.NewDefaultPathOptions()
	if f.fileOverride != "" {
		pathOptions = &clientcmd.PathOptions{
			GlobalFile: f.fileOverride,
		}
	}

	if err := f.modifyConfig(pathOptions, *ctpConf, false); err != nil {
		return err
	}

	if err := f.writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}
	return nil
}

// loadOutputKubeconfig loads the Kubeconfig that will be overwritten by the
// action, either loading it from the file override or defaulting back to the
// current kubeconfig
func (f *fileWriter) loadOutputKubeconfig() (config *clientcmdapi.Config, err error) {
	if f.fileOverride != "" {
		config, err = clientcmd.LoadFromFile(f.fileOverride)
		if errors.Is(err, fs.ErrNotExist) {
			return clientcmdapi.NewConfig(), nil
		} else if err != nil {
			return nil, err
		}
		return config, nil
	}

	raw, err := f.upCtx.Kubecfg.RawConfig()
	if err != nil {
		return nil, err
	}
	return &raw, nil
}

// upsertContext upserts the input kubeconfig based on its current context into
// the output kubeconfig at the destination context name set in the writer
func (f *fileWriter) upsertContext(inConfig *clientcmdapi.Config, outConfig *clientcmdapi.Config) (ctpConf *clientcmdapi.Config, prevContext string, err error) {
	// assumes the current context
	ctpConf, prevContext, err = mergeUpboundContext(outConfig, inConfig, inConfig.CurrentContext, f.kubeContext)
	if err != nil {
		return nil, "", err
	}
	if contextDeepEqual(ctpConf, prevContext, ctpConf.CurrentContext) {
		return nil, prevContext, nil
	}
	if err := f.verify(ctpConf); err != nil {
		return nil, "", err
	}

	return ctpConf, prevContext, nil
}

// mergeUpboundContext copies the provided group context into the config under
// the provided context name, updates the current context to the new context and
// renames the previous current context.
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
		// update all clusters using the existing previous to point at the new
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
		dest.Contexts[destContext] = ptr.To(*dest.Contexts[destContext+upboundPreviousContextSuffix])
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
