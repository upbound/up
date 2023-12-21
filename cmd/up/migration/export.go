package migration

import (
	"context"
	"fmt"
	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/export"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

type exportCmd struct {
}

func (c *exportCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	fmt.Println("Exporting ...")

	cfg := migCtx.Kubeconfig

	crdClient, err := apiextensionsclientset.NewForConfig(cfg)
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	exporter := export.NewControlPlaneStateExporter(crdClient, dynamicClient, mapper, export.Options{
		// TODO(turkenh): Pass these options from the CLI.
		ExcludedNamespaces: []string{"kube-system", "kube-public", "kube-node-lease", "local-path-storage"},
		IncludedResources:  []string{"namespaces", "configmaps", "secrets"}, // + all Crossplane resources
	})
	if err = exporter.Export(ctx); err != nil {
		return err
	}

	return nil
}
