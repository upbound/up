package importer

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceImporter interface {
	ImportResources(ctx context.Context, gvr schema.GroupVersionResource) error
}

type UnstructuredImporter struct {
	getter  ResourceGetter
	applier ResourceApplier
}

func NewUnstructuredImporter(g ResourceGetter, a ResourceApplier) *UnstructuredImporter {
	return &UnstructuredImporter{
		getter:  g,
		applier: a,
	}
}

func (i *UnstructuredImporter) ImportResources(ctx context.Context, gr schema.GroupResource) error {
	resources, err := i.getter.GetResources(gr.String())
	if err != nil {
		return errors.Wrapf(err, "cannot get %q resources", gr.String())
	}
	if err := i.applier.ApplyResources(ctx, resources); err != nil {
		return errors.Wrapf(err, "cannot apply %q resources", gr.String())
	}

	return nil
}
