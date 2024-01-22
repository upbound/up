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

package exporter

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceExporter interface {
	ExportResources(ctx context.Context, gvr schema.GroupVersionResource) (count int, err error)
}

type UnstructuredExporter struct {
	fetcher   ResourceFetcher
	persister ResourcePersister
}

func NewUnstructuredExporter(f ResourceFetcher, p ResourcePersister) *UnstructuredExporter {
	return &UnstructuredExporter{
		fetcher:   f,
		persister: p,
	}
}

func (e *UnstructuredExporter) ExportResources(ctx context.Context, gvr schema.GroupVersionResource) (int, error) {
	resources, err := e.fetcher.FetchResources(ctx, gvr)
	if err != nil {
		return 0, errors.Wrap(err, "cannot fetch resources")
	}

	for i := range resources {
		if err := cleanupClusterSpecificData(&resources[i]); err != nil {
			return 0, errors.Wrap(err, "cannot cleanup cluster specific data")
		}
	}

	if err = e.persister.PersistResources(ctx, gvr.GroupResource().String(), resources); err != nil {
		return 0, errors.Wrap(err, "cannot persist resources")
	}

	return len(resources), nil
}

func cleanupClusterSpecificData(u *unstructured.Unstructured) error {
	paved := fieldpath.Pave(u.Object)

	// Remove cluster specific data. Similar to Velero: https://github.com/vmware-tanzu/velero/blob/a81e049d362557c311cf8615c2c9c8bf77edf969/pkg/restore/restore.go#L2045
	for _, f := range []string{"generateName", "selfLink", "uid", "resourceVersion", "generation", "creationTimestamp", "ownerReferences", "managedFields"} {
		err := paved.DeleteField(fmt.Sprintf("metadata.%s", f))
		if err != nil {
			return errors.Wrapf(err, "cannot delete %q field", f)
		}
	}

	return nil
}
