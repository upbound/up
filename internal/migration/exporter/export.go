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
	"strings"

	"github.com/mholt/archiver/v4"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Options struct {
	OutputArchive string // default: xp-state.tar.gz

	IncludedNamespaces []string // default: none
	ExcludedNamespaces []string // default: except kube-system, kube-public, kube-node-lease, local-path-storage

	IncludedResources []string // default: namespaces, configmaps, secrets ( + all Crossplane resources)
	ExcludedResources []string // default: none
}

type ControlPlaneStateExporter struct {
	crdClient      apiextensionsclientset.Interface
	dynamicClient  dynamic.Interface
	resourceMapper meta.RESTMapper

	options Options
}

func NewControlPlaneStateExporter(crdClient apiextensionsclientset.Interface, dynamicClient dynamic.Interface, mapper meta.RESTMapper, opts Options) *ControlPlaneStateExporter {
	return &ControlPlaneStateExporter{
		crdClient:      crdClient,
		dynamicClient:  dynamicClient,
		resourceMapper: mapper,

		options: opts,
	}
}

func (e *ControlPlaneStateExporter) Export(ctx context.Context) error {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	tmpDir, err := fs.TempDir("", "up")
	if err != nil {
		return errors.Wrap(err, "cannot create temporary directory")
	}
	defer func() {
		_ = fs.RemoveAll(tmpDir)
	}()

	// Export native resources.
	for _, r := range e.options.IncludedResources {
		// TODO(turkenh): Proper parsing / resolving resources to GVRs.
		gvr := schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: r,
		}
		exporter := NewUnstructuredExporter(
			NewUnstructuredFetcher(e.dynamicClient, e.options),
			NewFileSystemPersister(fs, tmpDir, nil))

		if err = exporter.ExportResources(ctx, gvr); err != nil {
			return errors.Wrapf(err, "cannot export resources for %q", r)
		}
	}

	// Export custom resources.
	crdList, err := fetchAllCRDs(ctx, e.crdClient)
	if err != nil {
		return errors.Wrap(err, "cannot fetch CRDs")
	}
	for _, crd := range crdList {
		if !e.shouldExport(crd) {
			// Ignore CRDs that we don't want to export.
			continue
		}

		gvr, err := e.customResourceGVR(crd)
		if err != nil {
			return errors.Wrapf(err, "cannot get GVR for %q", crd.GetName())
		}

		exporter := NewUnstructuredExporter(
			NewUnstructuredFetcher(e.dynamicClient, e.options),
			NewFileSystemPersister(fs, tmpDir, crd.Spec.Names.Categories))

		if err = exporter.ExportResources(ctx, gvr); err != nil {
			return errors.Wrapf(err, "cannot export resources for %q", crd.GetName())
		}
	}

	// Archive the exported state.
	return e.archive(ctx, fs, tmpDir)
}

func (e *ControlPlaneStateExporter) shouldExport(in apiextensionsv1.CustomResourceDefinition) bool {
	for _, ref := range in.GetOwnerReferences() {
		if ref.APIVersion == "pkg.crossplane.io/v1" {
			// Note: We could also check the kind and ensure it is owned by a
			// Provider, Function or Configuration. However, this should be
			// enough and would be forward compatible if we introduce additional
			// package types.
			return true
		}

		if ref.APIVersion == "apiextensions.crossplane.io/v1" && ref.Kind == "CompositeResourceDefinition" {
			return true
		}
	}

	if strings.HasSuffix(in.GetName(), ".crossplane.io") {
		// Covering all built-in Crossplane CRDs.
		return true
	}

	for _, r := range e.options.IncludedResources {
		if in.GetName() == r {
			// If there are any extra CRs that we want to export.
			return true
		}
	}

	return false
}

func (e *ControlPlaneStateExporter) customResourceGVR(in apiextensionsv1.CustomResourceDefinition) (schema.GroupVersionResource, error) {
	versions := make([]string, 0, len(in.Spec.Versions))
	for _, vr := range in.Spec.Versions {
		versions = append(versions, vr.Name)
	}

	rm, err := e.resourceMapper.RESTMapping(schema.GroupKind{
		Group: in.Spec.Group,
		Kind:  in.Spec.Names.Kind,
	}, versions...)

	if err != nil {
		return schema.GroupVersionResource{}, errors.Wrapf(err, "cannot get REST mapping for %q", in.GetName())
	}

	return rm.Resource, nil
}

func (e *ControlPlaneStateExporter) archive(ctx context.Context, fs afero.Afero, dir string) error {
	files, err := archiver.FilesFromDisk(nil, map[string]string{
		dir + "/": "",
	})
	if err != nil {
		return err
	}

	out, err := fs.Create(e.options.OutputArchive)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if err = fs.Chmod(e.options.OutputArchive, 0600); err != nil {
		return err
	}

	format := archiver.CompressedArchive{
		Compression: archiver.Gz{},
		Archival:    archiver.Tar{},
	}

	return format.Archive(ctx, out, files)
}

func fetchAllCRDs(ctx context.Context, kube apiextensionsclientset.Interface) ([]apiextensionsv1.CustomResourceDefinition, error) {
	var crds []apiextensionsv1.CustomResourceDefinition

	continueToken := ""
	for {
		l, err := kube.ApiextensionsV1().CustomResourceDefinitions().List(ctx, v1.ListOptions{
			Limit:    defaultPageSize,
			Continue: continueToken,
		})
		if err != nil {
			return nil, errors.Wrap(err, "cannot list CRDs")
		}
		for _, r := range l.Items {
			crds = append(crds, r)
		}
		continueToken = l.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return crds, nil
}
