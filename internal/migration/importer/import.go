package importer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"os"
)

type Options struct {
	InputArchive string // default: xp-state.tar.gz
}

type ControlPlaneStateImporter struct {
	dynamicClient  dynamic.Interface
	resourceMapper meta.RESTMapper

	options Options
}

func NewControlPlaneStateImporter(dynamicClient dynamic.Interface, mapper meta.RESTMapper, opts Options) *ControlPlaneStateImporter {
	return &ControlPlaneStateImporter{
		dynamicClient:  dynamicClient,
		resourceMapper: mapper,
		options:        opts,
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

	fs := afero.Afero{Fs: tarfs.New(tar.NewReader(ur))}

	importer := NewUnstructuredImporter(NewFileSystemGetter(fs), NewTransformingResourceApplier(i.dynamicClient, i.resourceMapper, func(u *unstructured.Unstructured) {
		paved := fieldpath.Pave(u.Object)

		for _, f := range []string{"generateName", "selfLink", "uid", "resourceVersion", "generation", "creationTimestamp", "ownerReferences", "managedFields"} {
			err = paved.DeleteField(fmt.Sprintf("metadata.%s", f))
			if err != nil {
				// TODO(turkenh): proper error handling
				panic(err)
			}
		}
	}))

	for _, gr := range []string{"namespaces", "configmaps", "secrets", "storeconfigs.secrets.crossplane.io", "deploymentruntimeconfigs.pkg.crossplane.io", "providers.pkg.crossplane.io", "compositionrevisions.apiextensions.crossplane.io", "compositions.apiextensions.crossplane.io", "compositeresourcedefinitions.apiextensions.crossplane.io"} {
		if err = importer.ImportResources(ctx, schema.ParseGroupResource(gr)); err != nil {
			return errors.Wrapf(err, "cannot import %q resources", gr)
		}
	}

	// TODO(turkenh): Wait until all packages and XRDs are installed and healthy.

	// TODO(turkenh): Import the rest of the resources.

	return nil
}
