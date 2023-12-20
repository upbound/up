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
	PersistResources(ctx context.Context, resources []unstructured.Unstructured) error
}

type FileSystemPersister struct {
	fs   afero.Afero
	root string

	dir        string
	categories []string
}

func NewFileSystemPersister(fs afero.Afero, root string, dir string, categories []string) *FileSystemPersister {
	return &FileSystemPersister{
		fs:         fs,
		root:       root,
		dir:        dir,
		categories: categories,
	}
}

func (p *FileSystemPersister) PersistResources(_ context.Context, resources []unstructured.Unstructured) error {
	for _, c := range p.categories {
		if err := p.fs.MkdirAll(filepath.Join(p.root, "_categories", c), 0700); err != nil {
			return errors.Wrapf(err, "cannot create directory %q for category %q", filepath.Join(p.root, c), c)
		}
	}

	for _, r := range resources {
		if err := p.fs.MkdirAll(filepath.Join(p.root, p.dir, r.GetNamespace()), 0700); err != nil {
			return errors.Wrapf(err, "cannot create directory %q for resource %q", p.dir, r.GetName())
		}

		linker, ok := p.fs.Fs.(afero.Linker)
		if ok {
			for _, c := range p.categories {
				if err := linker.SymlinkIfPossible(filepath.Join("../..", p.dir), filepath.Join(p.root, "_categories", c, p.dir)); err != nil {
					return errors.Wrapf(err, "cannot create symlink for resources %q in category %q", p.dir, c)
				}
			}
		}

		dir := p.dir
		if r.GetNamespace() != "" {
			dir = filepath.Join(dir, r.GetNamespace())
		}

		b, err := yaml.Marshal(&r)
		if err != nil {
			return errors.Wrap(err, "cannot marshal resource to yaml")
		}

		f := filepath.Join(p.root, dir, r.GetName()+".yaml")
		err = p.fs.WriteFile(f, b, 0600)
		if err != nil {
			return errors.Wrapf(err, "cannot write resource to %q", f)
		}
	}

	return nil
}
