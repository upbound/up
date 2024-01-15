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
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const categoryPattern = `^[a-z]([-a-z0-9]*[a-z0-9])?$`
const yamlPathPattern = `^(cluster|namespaces\/[a-z0-9]([-a-z0-9]*[a-z0-9])?)\/[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\.yaml$`

var (
	categoryRegex = regexp.MustCompile(categoryPattern)
	yamlPathRegex = regexp.MustCompile(yamlPathPattern)
)

type ResourceReader interface {
	ReadResources(groupResource string) (categories []string, resources []unstructured.Unstructured, err error)
}

type FileSystemReader struct {
	fs afero.Afero
}

// Directory structure:
// <groupResource>/<cluster or namespace>/<?namespace>/<name>.yaml
// <groupResource>/<category1> (an empty file)
// <groupResource>/<category2> (an empty file)
// <groupResource>/<categoryN> (an empty file)

func NewFileSystemReader(fs afero.Afero) *FileSystemReader {
	return &FileSystemReader{
		fs: fs,
	}
}

func (g *FileSystemReader) ReadResources(groupResource string) (categories []string, resources []unstructured.Unstructured, rErr error) {
	rErr = g.fs.Walk(groupResource, func(path string, info fs.FileInfo, _ error) error {
		if info == nil || info.IsDir() {
			return nil
		}

		groupPath := strings.TrimPrefix(path, groupResource+string(os.PathSeparator))
		if categoryRegex.MatchString(groupPath) {
			categories = append(categories, groupPath)
			return nil
		}

		if !yamlPathRegex.MatchString(groupPath) {
			return errors.Errorf("invalid path %q for YAML file, should match regexp %q", groupPath, yamlPathPattern)
		}

		b, err := g.fs.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "cannot read file %q", path)
		}

		var r unstructured.Unstructured
		if err := yaml.Unmarshal(b, &r); err != nil {
			return errors.Wrapf(err, "cannot unmarshal file %q", path)
		}

		resources = append(resources, r)
		return nil
	})
	if rErr != nil {
		return nil, nil, errors.Wrapf(rErr, "cannot walk directory for resource group %q", groupResource)
	}

	return categories, resources, nil
}
