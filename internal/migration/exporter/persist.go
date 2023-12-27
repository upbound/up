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
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type ResourcePersister interface {
	PersistResources(ctx context.Context, groupResource string, resources []unstructured.Unstructured) error
}

type FileSystemPersister struct {
	fs   afero.Afero
	root string

	categories []string
}

func NewFileSystemPersister(fs afero.Afero, root string, categories []string) *FileSystemPersister {
	return &FileSystemPersister{
		fs:         fs,
		root:       root,
		categories: categories,
	}
}

func (p *FileSystemPersister) pathFor(dirs ...string) string {
	dirs = append([]string{p.root}, dirs...)
	return filepath.Join(dirs...)
}

func (p *FileSystemPersister) PersistResources(_ context.Context, groupResource string, resources []unstructured.Unstructured) error {
	if len(resources) == 0 {
		return nil
	}

	if err := p.fs.MkdirAll(p.pathFor(groupResource), 0700); err != nil {
		return errors.Wrapf(err, "cannot create directory resource group", groupResource)
	}

	for _, c := range p.categories {
		f, err := p.fs.OpenFile(p.pathFor(groupResource, c), os.O_RDONLY|os.O_CREATE, 0600)
		if err != nil {
			return errors.Wrapf(err, "cannot touch category file %q", c)
		}
		_ = f.Close()
	}

	for _, r := range resources {
		fileDirPath := p.pathFor(groupResource, "cluster")
		if r.GetNamespace() != "" {
			fileDirPath = p.pathFor(groupResource, "namespaces", r.GetNamespace())
		}

		if err := p.fs.MkdirAll(fileDirPath, 0700); err != nil {
			return errors.Wrapf(err, "cannot create directory %q for resource %q", groupResource, r.GetName())
		}

		b, err := yaml.Marshal(&r)
		if err != nil {
			return errors.Wrap(err, "cannot marshal resource to yaml")
		}

		f := filepath.Join(fileDirPath, r.GetName()+".yaml")
		err = p.fs.WriteFile(f, b, 0600)
		if err != nil {
			return errors.Wrapf(err, "cannot write resource to %q", f)
		}
	}

	return nil
}
