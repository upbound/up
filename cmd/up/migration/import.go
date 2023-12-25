package migration

import (
	"context"
	"fmt"
	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/importer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

type importCmd struct {
}

func (c *importCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	fmt.Println("Importing ...")

	cfg := migCtx.Kubeconfig
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	i := importer.NewControlPlaneStateImporter(dynamicClient, mapper, importer.Options{
		InputArchive: "xp-state.tar.gz",
	})
	if err = i.Import(ctx); err != nil {
		return err
	}

	fmt.Println("Import Complete!")
	return nil
}
