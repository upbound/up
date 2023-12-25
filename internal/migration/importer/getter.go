package importer

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"io/fs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"regexp"
	"sigs.k8s.io/yaml"
	"strings"
)

const categoryPattern = `^_categories\/[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
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

func (g *FileSystemReader) ReadResources(groupResource string) (categories []string, resources []unstructured.Unstructured, err error) {
	err = g.fs.Walk(groupResource, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
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
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot walk directory for resource group %q", groupResource)
	}

	return nil, resources, nil
}
