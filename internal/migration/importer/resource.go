package importer

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

type ResourceImporter interface {
	ImportResources(ctx context.Context) error
}

type UnstructuredImporter struct {
	getter ResourceGetter
	//applier ResourceApplier
}

func NewUnstructuredImporter(g ResourceGetter) *UnstructuredImporter {
	return &UnstructuredImporter{
		getter: g,
		//applier: a,
	}
}

func (i *UnstructuredImporter) ImportResources(ctx context.Context) error {
	providers, err := i.getter.GetResourcesWithCategory("managed")
	if err != nil {
		return errors.Wrap(err, "cannot get providers")
	}

	for _, p := range providers {
		fmt.Println(p.GetName())
	}

	return nil
}
