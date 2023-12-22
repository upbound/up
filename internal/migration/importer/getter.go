package importer

import (
	"archive/tar"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"io/fs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/yaml"
	"strings"
)

const pathPattern = `^(cluster|namespaces\/[a-z0-9]([-a-z0-9]*[a-z0-9])?)\/[a-z0-9]([-a-z0-9]*[a-z0-9])?\.yaml$`

var (
	pathRegex = regexp.MustCompile(pathPattern)
)

type ResourceGetter interface {
	GetResources(groupResource string) ([]unstructured.Unstructured, error)
	GetResourcesWithCategory(category string) ([]unstructured.Unstructured, error)
}

type FileSystemGetter struct {
	fs afero.Afero
}

// Directory structure:
// _categories/<category>/<groupResource>/<"cluster" or "namespace">/<?namespace>/<name>.yaml
// <groupResource>/<cluster or namespace>/<?namespace>/<name>.yaml

func NewFileSystemGetter(fs afero.Afero) *FileSystemGetter {
	return &FileSystemGetter{
		fs: fs,
	}
}

func validYAMLPath(path string) bool {
	return pathRegex.MatchString(path)
}

func (g *FileSystemGetter) GetResources(groupResource string) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured

	err := g.fs.Walk(groupResource, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		groupPath := strings.TrimPrefix(path, groupResource+string(os.PathSeparator))
		if !validYAMLPath(groupPath) {
			return errors.Errorf("invalid path %q for file, should match regexp %q", groupPath, pathPattern)
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
	if err != nil {
		return nil, errors.Wrapf(err, "cannot walk directory for resource group %q", groupResource)
	}

	return resources, nil
}

func (g *FileSystemGetter) GetResourcesWithCategory(category string) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured

	categoryPath := filepath.Join("_categories", category)
	err := g.fs.Walk(categoryPath, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if info.Mode()&fs.ModeSymlink != fs.ModeSymlink {
			return errors.Errorf("unexpected file %q in category %q, should be a symlink", path, category)
		}

		// TODO(turkenh): Use afero.LinkReader to read symlinks.
		//  This is not supported for tarfs and needs to be implemented in afero.
		//  Below is a workaround to read the symlink target and we're leaking the tarfs implementation.
		th, ok := info.Sys().(*tar.Header)
		if !ok {
			return errors.Errorf("unexpected file %q in category %q, should be a tar header", path, category)
		}

		newPath := filepath.Join(filepath.Dir(path), th.Linkname)
		r, err := g.GetResources(newPath)
		if err != nil {
			return errors.Wrapf(err, "cannot get resources for path %q", newPath)
		}

		resources = append(resources, r...)
		return nil
	})

	if err != nil {
		return nil, errors.Wrapf(err, "cannot walk directory for category %q", category)
	}

	return resources, nil
}
