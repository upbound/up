// Copyright 2024 Upbound Inc
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
)

type ResourceImporter interface {
	ImportResources(ctx context.Context, gr string, restoreStatus bool) (int, error)
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

func (im *PausingResourceImporter) ImportResources(ctx context.Context, gr string, restoreStatus bool) (int, error) {
	resources, typeMeta, err := im.reader.ReadResources(gr)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot get %q resources", gr)
	}

	hasSubresource := false
	if typeMeta != nil {
		hasSubresource = typeMeta.WithStatusSubresource
		for _, c := range typeMeta.Categories {
			// We pause all resources that are managed, claim, or composite.
			// - Claim/Composite: We don't want Crossplane controllers to create new resources before we import all.
			// - Managed: Same reason as above, but also don't want to take control of cloud resources yet.
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

	if err = im.applier.ApplyResources(ctx, resources, restoreStatus && hasSubresource); err != nil {
		return 0, errors.Wrapf(err, "cannot apply %q resources", gr)
	}

	return len(resources), nil
}
