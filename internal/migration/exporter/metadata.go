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

package exporter

import (
	"context"
	"path/filepath"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/upbound/up/internal/migration/crossplane"
	"github.com/upbound/up/internal/migration/meta/v1alpha1"
)

type MetadataExporter interface {
	ExportMetadata(ctx context.Context) error
}

type PersistentMetadataExporter struct {
	appsClient appsv1.AppsV1Interface
	fs         afero.Afero
	root       string
}

func NewPersistentMetadataExporter(apps appsv1.AppsV1Interface, fs afero.Afero, root string) *PersistentMetadataExporter {
	return &PersistentMetadataExporter{
		appsClient: apps,
		fs:         fs,
		root:       root,
	}
}

func (e *PersistentMetadataExporter) ExportMetadata(ctx context.Context, opts Options, native map[string]int, custom map[string]int) error {
	xp, err := crossplane.CollectInfo(ctx, e.appsClient)
	if err != nil {
		return errors.Wrap(err, "cannot get Crossplane info")
	}

	total := 0
	for _, v := range native {
		total += v
	}
	for _, v := range custom {
		total += v
	}
	em := &v1alpha1.ExportMeta{
		Version:    "v1alpha1",
		ExportedAt: time.Now(),
		Options: v1alpha1.ExportOptions{
			IncludedNamespaces:     opts.IncludeNamespaces,
			ExcludedNamespaces:     opts.ExcludeNamespaces,
			IncludedExtraResources: opts.IncludeExtraResources,
			ExcludedResources:      opts.ExcludeResources,
			PausedBeforeExport:     opts.PauseBeforeExport,
		},
		Crossplane: *xp,
		Stats: v1alpha1.ExportStats{
			Total:           total,
			NativeResources: native,
			CustomResources: custom,
		},
	}
	b, err := yaml.Marshal(&em)
	if err != nil {
		return errors.Wrap(err, "cannot marshal export metadata to yaml")
	}
	err = e.fs.WriteFile(filepath.Join(e.root, "export.yaml"), b, 0600)
	if err != nil {
		return errors.Wrap(err, "cannot write export metadata")
	}
	return nil
}
