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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/meta"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

type ResourceImporter interface {
	ImportResources(ctx context.Context, gr string) (int, error)
	UnpauseResources(ctx context.Context, gr string, withCategories []string) (int, error)
}

type PausingResourceImporter struct {
	reader  ResourceReader
	applier ResourceApplier
}

func NewPausingResourceImporter(r ResourceReader, a ResourceApplier) *PausingResourceImporter {
	return &PausingResourceImporter{
		reader:  r,
		applier: a,
	}
}

func (im *PausingResourceImporter) ImportResources(ctx context.Context, gr string) (int, error) {
	resources, typeMeta, err := im.reader.ReadResources(gr)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot get %q resources", gr)
	}

	sub := false
	if typeMeta != nil {
		sub = typeMeta.WithStatusSubresource
		for _, c := range typeMeta.Categories {
			if c == "managed" || c == "claim" || c == "composite" {
				for i := range resources {
					meta.AddAnnotations(&resources[i], map[string]string{
						"crossplane.io/paused": "true",
					})
				}
				break
			}
		}
	}

	if err = im.applier.ApplyResources(ctx, resources, sub); err != nil {
		return 0, errors.Wrapf(err, "cannot apply %q resources", gr)
	}

	return len(resources), nil
}

func (im *PausingResourceImporter) UnpauseResources(ctx context.Context, gr string, withCategories []string) (int, error) {
	resources, typeMeta, err := im.reader.ReadResources(gr)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot get %q resources", gr)
	}

	match := false
	if typeMeta != nil {
		for _, wc := range withCategories {
			for _, c := range typeMeta.Categories {
				if c == wc {
					match = true
					break
				}
			}
		}
	}

	if !match {
		return 0, nil
	}

	// Resources contains the manifests from the export, which doesn't have the paused annotations we added during
	// import. So, we just need to apply it to get the annotations we added to be removed.
	if err = im.applier.ModifyResources(ctx, resources, func(u *unstructured.Unstructured) error {
		meta.RemoveAnnotations(u, "crossplane.io/paused")
		return nil
	}); err != nil {
		return 0, errors.Wrapf(err, "cannot apply %q resources to get paused annotations removed", gr)
	}

	return len(resources), nil
}
