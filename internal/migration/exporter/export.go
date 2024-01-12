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
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/mholt/archiver/v4"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up/internal/upterm"
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
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	pterm.Info.Println("Exporting control plane state...")

	fs := afero.Afero{Fs: afero.NewOsFs()}
	tmpDir, err := fs.TempDir("", "up")
	if err != nil {
		return errors.Wrap(err, "cannot create temporary directory")
	}
	defer func() {
		_ = fs.RemoveAll(tmpDir)
	}()

	// Scan the control plane for types to export.
	scanMsg := "Scanning control plane for types to export... "
	s, _ := upterm.EyesInfoSpinner.Start(scanMsg)
	crdList, err := fetchAllCRDs(ctx, e.crdClient)
	if err != nil {
		s.Fail(scanMsg + "Failed!")
		return errors.Wrap(err, "cannot fetch CRDs")
	}
	exportList := make([]apiextensionsv1.CustomResourceDefinition, 0, len(crdList))
	for _, crd := range crdList {
		if !e.shouldExport(crd) {
			// Ignore CRDs that we don't want to export.
			continue
		}
		exportList = append(exportList, crd)
	}
	s.Info(scanMsg + fmt.Sprintf("%d types found!", len(exportList)))
	//////////////////////

	// Export Crossplane resources.
	crCounts := make(map[string]int, len(crdList))
	exportCRsMsg := "Exporting Crossplane resources... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(exportCRsMsg + fmt.Sprintf("0 / %d", len(crdList)))
	for i, crd := range crdList {
		gvr, err := e.customResourceGVR(crd)
		if err != nil {
			s.Fail(exportCRsMsg + "Failed!")
			return errors.Wrapf(err, "cannot get GVR for %q", crd.GetName())
		}

		s.UpdateText(fmt.Sprintf("(%d / %d) Exporting %s...", i, len(crdList), gvr.GroupResource()))

		exporter := NewUnstructuredExporter(
			NewUnstructuredFetcher(e.dynamicClient, e.options),
			NewFileSystemPersister(fs, tmpDir, crd.Spec.Names.Categories))

		count, err := exporter.ExportResources(ctx, gvr)
		if err != nil {
			s.Fail(exportCRsMsg + "Failed!")
			return errors.Wrapf(err, "cannot export resources for %q", crd.GetName())
		}
		crCounts[gvr.GroupResource().String()] = count
	}

	total := 0
	for _, count := range crCounts {
		total += count
	}
	s.Success(exportCRsMsg + fmt.Sprintf("%d resources exported!", total))
	//////////////////////

	// Export native resources.
	exportNativeMsg := "Exporting native resources... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(exportNativeMsg + fmt.Sprintf("0 / %d", len(e.options.IncludedResources)))
	nativeCounts := make(map[string]int, len(e.options.IncludedResources))
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

		count, err := exporter.ExportResources(ctx, gvr)
		if _, err = exporter.ExportResources(ctx, gvr); err != nil {
			s.Fail(exportNativeMsg + "Failed!")
			return errors.Wrapf(err, "cannot export resources for %q", r)
		}
		nativeCounts[gvr.Resource] = count
	}
	total = 0
	for _, count := range nativeCounts {
		total += count
	}
	s.Success(exportNativeMsg + fmt.Sprintf("%d resources exported!", total))
	//////////////////////

	// Archive the exported state.
	archiveMsg := "Archiving exported state... "
	s, _ = upterm.ArchiveSuccessSpinner.Start(archiveMsg)
	if err = e.archive(ctx, fs, tmpDir); err != nil {
		s.Fail(archiveMsg + "Failed!")
		return errors.Wrap(err, "cannot archive exported state")
	}
	s.Success(archiveMsg + fmt.Sprintf("archived to %q!", e.options.OutputArchive))
	//////////////////////

	pterm.Success.Println("Successfully exported control plane state!")
	return nil
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
