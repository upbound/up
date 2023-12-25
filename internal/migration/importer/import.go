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

var (
	coreResources = map[string]struct{}{
		// Core Kubernetes resources
		"namespaces": {},
		"configmaps": {},
		"secrets":    {},

		// Crossplane resources
		// Runtime
		"controllerconfigs.pkg.crossplane.io":        {},
		"deploymentruntimeconfigs.pkg.crossplane.io": {},
		"storeconfigs.secrets.crossplane.io":         {},
		// Compositions
		"compositionrevisions.apiextensions.crossplane.io":         {},
		"compositions.apiextensions.crossplane.io":                 {},
		"compositeresourcedefinitions.apiextensions.crossplane.io": {},
		// Packages
		"providers.pkg.crossplane.io":      {},
		"functions.pkg.crossplane.io":      {},
		"configurations.pkg.crossplane.io": {},
	}
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

	r := NewTransformingResourceImporter(NewFileSystemReader(fs), NewUnstructuredResourceApplier(i.dynamicClient, i.resourceMapper), []ResourceTransformer{
		func(u *unstructured.Unstructured) error {
			paved := fieldpath.Pave(u.Object)

			for _, f := range []string{"generateName", "selfLink", "uid", "resourceVersion", "generation", "creationTimestamp", "ownerReferences", "managedFields"} {
				err = paved.DeleteField(fmt.Sprintf("metadata.%s", f))
				if err != nil {
					return errors.Wrapf(err, "cannot delete %q field", f)
				}
			}
			return nil
		},
	})

	for gr := range coreResources {
		if err = r.ImportResources(ctx, schema.ParseGroupResource(gr)); err != nil {
			return errors.Wrapf(err, "cannot import %q resources", gr)
		}
	}

	// TODO(turkenh): Wait until all packages and XRDs are installed and healthy.

	// TODO(turkenh): Import the rest of the resources.
	grs, err := fs.ReadDir("/")
	if err != nil {
		return errors.Wrap(err, "cannot list group resources")
	}
	for _, gr := range grs {
		if !gr.IsDir() {
			return errors.Errorf("unexpected file %q in root directory of exported state", gr.Name())
		}

		if _, ok := coreResources[gr.Name()]; ok {
			// We already imported core resources above.
			continue
		}

		if err = r.ImportResources(ctx, schema.ParseGroupResource(gr.Name())); err != nil {
			return errors.Wrapf(err, "cannot import %q resources", gr.Name())
		}
	}

	return nil
}
