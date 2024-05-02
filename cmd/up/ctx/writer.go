package ctx

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"

	"github.com/upbound/up/internal/kube"
	upbound "github.com/upbound/up/internal/upbound"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
)

type kubeContextWriter interface {
	Write(upCtx *upbound.Context, config *clientcmdapi.Config) error
}

type printWriter struct {
}

var _ kubeContextWriter = &printWriter{}

// Write implements kubeconfigWriter.
func (p *printWriter) Write(upCtx *upbound.Context, config *clientcmdapi.Config) error {
	b, err := clientcmd.Write(*config)
	if err != nil {
		return err
	}

	fmt.Print(string(b))
	return nil
}

type fileWriter struct {
	fileOverride string
	kubeContext  string
}

var _ kubeContextWriter = &fileWriter{}

// Write implements kubeconfigWriter.
func (f *fileWriter) Write(upCtx *upbound.Context, config *clientcmdapi.Config) error {
	outConfig, err := f.loadOutputKubeconfig(upCtx)
	if err != nil {
		return err
	}

	ctpConf, prevContext, err := f.replaceContext(upCtx, config, outConfig)
	if err != nil {
		return err
	}

	pathOptions := clientcmd.NewDefaultPathOptions()
	if f.fileOverride != "" {
		pathOptions = &clientcmd.PathOptions{
			GlobalFile: f.fileOverride,
		}
	}

	if err := clientcmd.ModifyConfig(pathOptions, *ctpConf, false); err != nil {
		return err
	}

	if err := writeLastContext(prevContext); err != nil { // nolint:staticcheck
		// ignore error because now everything has happened already.
	}
	return nil
}

// loadOutputKubeconfig loads the Kubeconfig that will be overwritten by the
// action, either loading it from the file override or defaulting back to the
// current kubeconfig
func (f *fileWriter) loadOutputKubeconfig(upCtx *upbound.Context) (config *clientcmdapi.Config, err error) {
	if f.fileOverride != "" {
		config, err = clientcmd.LoadFromFile(f.fileOverride)
		if errors.Is(err, fs.ErrNotExist) {
			return clientcmdapi.NewConfig(), nil
		} else if err != nil {
			return nil, err
		}
		return config, nil
	}

	raw, err := upCtx.Kubecfg.RawConfig()
	if err != nil {
		return nil, err
	}
	return &raw, nil
}

// replaceContext upserts the current kubeconfig with the upserted upbound context
// and cluster to point to the given space and resource
func (f *fileWriter) replaceContext(upCtx *upbound.Context, inConfig *clientcmdapi.Config, outConfig *clientcmdapi.Config) (ctpConf *clientcmdapi.Config, prevContext string, err error) {
	// assumes the current context
	ctpConf, prevContext, err = mergeUpboundContext(outConfig, inConfig, inConfig.CurrentContext, f.kubeContext)
	if err != nil {
		return nil, "", err
	}
	if contextDeepEqual(ctpConf, prevContext, ctpConf.CurrentContext) {
		return nil, prevContext, nil
	}
	if err := kube.VerifyKubeConfig(upCtx.WrapTransport)(ctpConf); err != nil {
		return nil, "", err
	}

	return ctpConf, prevContext, nil
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
