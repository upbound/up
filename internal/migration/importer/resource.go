package importer

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceImporter interface {
	ImportResources(ctx context.Context, gvr schema.GroupVersionResource) error
}

type ResourceTransformer func(*unstructured.Unstructured) error

type TransformingResourceImporter struct {
	reader  ResourceReader
	applier ResourceApplier

	transformers []ResourceTransformer
}

func NewTransformingResourceImporter(r ResourceReader, a ResourceApplier, t []ResourceTransformer) *TransformingResourceImporter {
	return &TransformingResourceImporter{
		reader:       r,
		applier:      a,
		transformers: t,
	}
}

func (i *TransformingResourceImporter) ImportResources(ctx context.Context, gr schema.GroupResource) error {
	categories, resources, err := i.reader.ReadResources(gr.String())
	if err != nil {
		return errors.Wrapf(err, "cannot get %q resources", gr.String())
	}

	isManaged := false
	for _, c := range categories {
		if c == "managed" {
			isManaged = true
			break
		}
	}

	t := i.transformers
	if isManaged {
		t = append(t, func(r *unstructured.Unstructured) error {
			meta.AddAnnotations(r, map[string]string{
				"crossplane.io/paused": "true",
			})
			return nil
		})
	}

	for _, r := range resources {
		for _, t := range i.transformers {
			if err = t(&r); err != nil {
				return errors.Wrapf(err, "cannot transform resource %q for import", r.GetName())
			}
		}
	}

	if err := i.applier.ApplyResources(ctx, resources); err != nil {
		return errors.Wrapf(err, "cannot apply %q resources", gr.String())
	}

	return nil
}
