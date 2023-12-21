package export

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"path/filepath"
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
	linker, linkCategories := p.fs.Fs.(afero.Linker)
	if linkCategories {
		// Create directories for all categories if they don't exist.
		// We will only need the directories if we are going to create symlinks.
		for _, c := range p.categories {
			if err := p.fs.MkdirAll(p.pathFor("_categories", c), 0700); err != nil {
				return errors.Wrapf(err, "cannot create directory %q for category %q", filepath.Join(p.root, c), c)
			}
		}
	}

	for _, r := range resources {
		if err := p.fs.MkdirAll(p.pathFor(groupResource), 0700); err != nil {
			return errors.Wrapf(err, "cannot create directory %q for resource %q", groupResource, r.GetName())
		}

		if linkCategories {
			for _, c := range p.categories {
				if err := linker.SymlinkIfPossible(filepath.Join("../..", groupResource), p.pathFor("_categories", c, groupResource)); err != nil {
					return errors.Wrapf(err, "cannot create symlink for resources %q in category %q", groupResource, c)
				}
			}
		}

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
