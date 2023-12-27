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

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceExporter interface {
	ExportResources(ctx context.Context, gvr schema.GroupVersionResource) error
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

func (e *UnstructuredExporter) ExportResources(ctx context.Context, gvr schema.GroupVersionResource) error {
	resources, err := e.fetcher.FetchResources(ctx, gvr)
	if err != nil {
		return errors.Wrap(err, "cannot fetch resources")
	}

	if err = e.persister.PersistResources(ctx, gvr.GroupResource().String(), resources); err != nil {
		return errors.Wrap(err, "cannot persist resources")
	}

	return nil
}
