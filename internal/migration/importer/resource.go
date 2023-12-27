// Copyright 2023 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
