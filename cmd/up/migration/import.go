package migration

import (
	"context"
	"fmt"
	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/importer"
	"k8s.io/client-go/dynamic"
)

type importCmd struct {
}

func (c *importCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	fmt.Println("Importing ...")

	dynamicClient, err := dynamic.NewForConfig(migCtx.Kubeconfig)
	if err != nil {
		return err
	}

	i := importer.NewControlPlaneStateImporter(dynamicClient, importer.Options{
		InputArchive: "xp-state.tar.gz",
	})
	if err = i.Import(ctx); err != nil {
		return err
	}

	return nil
}
