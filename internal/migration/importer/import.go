package importer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"k8s.io/client-go/dynamic"
	"os"
)

type Options struct {
	InputArchive string // default: xp-state.tar.gz
}

type ControlPlaneStateImporter struct {
	dynamicClient dynamic.Interface

	options Options
}

func NewControlPlaneStateImporter(dynamicClient dynamic.Interface, opts Options) *ControlPlaneStateImporter {
	return &ControlPlaneStateImporter{
		dynamicClient: dynamicClient,
		options:       opts,
	}
}

func (i *ControlPlaneStateImporter) Import(ctx context.Context) error {
	g, err := os.Open(i.options.InputArchive)
	if err != nil {
		errors.Wrap(err, "cannot open input archive")
	}
	defer func() {
		_ = g.Close()
	}()

	ur, err := gzip.NewReader(g)
	if err != nil {
		return errors.Wrap(err, "cannot decompress archive")
	}
	defer func() {
		_ = ur.Close()
	}()

	_ = afero.Afero{Fs: tarfs.New(tar.NewReader(ur))}

	return nil
}
