// Copyright 2024 Upbound Inc
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
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/mholt/archiver/v4"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	"github.com/upbound/up/internal/migration/category"
	"github.com/upbound/up/internal/migration/crossplane"
	"github.com/upbound/up/internal/migration/meta/v1alpha1"
	"github.com/upbound/up/internal/upterm"
)

var (
	baseResources = []string{
		// Core Kubernetes resources
		"namespaces",
		"configmaps",
		"secrets",

		// Crossplane resources
		// Runtime
		"controllerconfigs.pkg.crossplane.io",
		"deploymentruntimeconfigs.pkg.crossplane.io",
		"storeconfigs.secrets.crossplane.io",
		// Compositions
		"compositionrevisions.apiextensions.crossplane.io",
		"compositions.apiextensions.crossplane.io",
		"compositeresourcedefinitions.apiextensions.crossplane.io",
		// Packages
		"providers.pkg.crossplane.io",
		"functions.pkg.crossplane.io",
		"configurations.pkg.crossplane.io",
	}
)

// Options are the options for the import command.
type Options struct {
	// InputArchive is the path to the archive to be imported.
	InputArchive string // default: xp-state.tar.gz
	// UnpauseAfterImport indicates whether to unpause all managed resources after import.
	UnpauseAfterImport bool // default: false
}

// ControlPlaneStateImporter is the importer for control plane state.
type ControlPlaneStateImporter struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	appsClient      appsv1.AppsV1Interface
	resourceMapper  meta.ResettableRESTMapper

	fs *afero.Afero

	options Options
}

// NewControlPlaneStateImporter creates a new importer for control plane state.
func NewControlPlaneStateImporter(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface, appsClient appsv1.AppsV1Interface, mapper meta.ResettableRESTMapper, opts Options) *ControlPlaneStateImporter {
	return &ControlPlaneStateImporter{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		appsClient:      appsClient,
		resourceMapper:  mapper,
		options:         opts,
	}
}

// Import imports the control plane state.
func (im *ControlPlaneStateImporter) Import(ctx context.Context) error { // nolint:gocyclo // This is the high level import command, so it's expected to be a bit complex.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	pterm.Println("Importing control plane state...")

	// Reading state from the archive
	unarchiveMsg := "Reading state from the archive... "
	s, _ := upterm.CheckmarkSuccessSpinner.Start(unarchiveMsg)

	// If preflight checks were already done, which unarchives to get the `export.yaml`, we don't need to do it again.
	if im.fs == nil {
		// We export the archive to a memory map file system. Assuming the archive is not too big
		// (a bunch of yaml files, this should be fine).
		im.fs = &afero.Afero{Fs: afero.NewMemMapFs()}

		if err := im.unarchive(ctx, *im.fs); err != nil {
			s.Fail(unarchiveMsg + "Failed!")
			return errors.Wrap(err, "cannot unarchive export archive")
		}
	}

	s.Success(unarchiveMsg + "Done! 👀")
	//////////////////////////////////////////

	// Pausing resource importer will import all resources.
	// It will import all Claims, Composites and Managed resource with the `crossplane.io/paused` annotation set to `true`.
	r := NewPausingResourceImporter(NewFileSystemReader(*im.fs), NewUnstructuredResourceApplier(im.dynamicClient, im.resourceMapper))

	// Import base resources which are defined with the `baseResources` variable.
	// They could be considered as the custom or native resources that do not depend on any packages (e.g. Managed Resources) or XRDs (e.g. Claims/Composites).
	// They are imported first to make sure that all the resources that depend on them can be imported at a later stage.
	importBaseMsg := "Importing base resources... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(importBaseMsg + fmt.Sprintf("0 / %d", len(baseResources)))
	baseCounts := make(map[string]int, len(baseResources))
	for i, gr := range baseResources {
		count, err := r.ImportResources(ctx, gr, false)
		if err != nil {
			s.Fail(importBaseMsg + "Failed!")
			return errors.Wrapf(err, "cannot import %q resources", gr)
		}
		s.UpdateText(fmt.Sprintf("(%d / %d) Importing %s...", i, len(baseResources), gr))
		baseCounts[gr] = count
	}
	total := 0
	for _, count := range baseCounts {
		total += count
	}
	s.Success(importBaseMsg + fmt.Sprintf("%d resources imported! 📥", total))
	//////////////////////////////////////////

	// Wait for all XRDs and Packages to be ready before importing the resources that depend on them.

	waitXRDsMsg := "Waiting for XRDs... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(waitXRDsMsg)
	if err := im.waitForConditions(ctx, s, schema.GroupKind{Group: "apiextensions.crossplane.io", Kind: "CompositeResourceDefinition"}, []xpv1.ConditionType{"Established"}); err != nil {
		s.Fail(waitXRDsMsg + "Failed!")
		return errors.Wrap(err, "there are unhealthy CompositeResourceDefinitions")
	}
	s.Success(waitXRDsMsg + "Established! ⏳")

	waitPkgsMsg := "Waiting for Packages... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(waitPkgsMsg)
	for _, k := range []schema.GroupKind{
		{Group: "pkg.crossplane.io", Kind: "Provider"},
		{Group: "pkg.crossplane.io", Kind: "Function"},
		{Group: "pkg.crossplane.io", Kind: "Configuration"},
	} {
		if err := im.waitForConditions(ctx, s, k, []xpv1.ConditionType{"Installed", "Healthy"}); err != nil {
			s.Fail(waitPkgsMsg + "Failed!")
			return errors.Wrapf(err, "there are unhealthy %qs", k.Kind)
		}
	}

	// Note(turkenh): We should not need to wait for ProviderRevision, FunctionRevision, and ConfigurationRevision.
	// Crossplane should not report packages as ready before revisions are healthy. This is a bug in Crossplane
	// version <1.14 which was fixed with https://github.com/crossplane/crossplane/pull/4647
	// Todo(turkenh): Remove these once Crossplane 1.13 is no longer supported.
	for _, k := range []schema.GroupKind{
		{Group: "pkg.crossplane.io", Kind: "ProviderRevision"},
		{Group: "pkg.crossplane.io", Kind: "FunctionRevision"},
		{Group: "pkg.crossplane.io", Kind: "ConfigurationRevision"},
	} {
		if err := im.waitForConditions(ctx, s, k, []xpv1.ConditionType{"Healthy"}); err != nil {
			s.Fail(waitPkgsMsg + "Failed!")
			return errors.Wrapf(err, "there are unhealthy %qs", k.Kind)
		}
	}

	s.Success(waitPkgsMsg + "Installed and Healthy! ⏳")
	//////////////////////////////////////////

	// Reset the resource mapper to make sure all CRDs introduced by packages or XRDs are available.
	im.resourceMapper.Reset()

	// Import remaining resources other than the base resources.
	importRemainingMsg := "Importing remaining resources... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(importRemainingMsg)
	grs, err := im.fs.ReadDir("/")
	if err != nil {
		s.Fail(importRemainingMsg + "Failed!")
		return errors.Wrap(err, "cannot list group resources")
	}
	remainingCounts := make(map[string]int, len(grs))
	for i, info := range grs {
		if info.Name() == "export.yaml" {
			// This is the top level export metadata file, so nothing to import.
			continue
		}
		if !info.IsDir() {
			return errors.Errorf("unexpected file %q in root directory of exported state", info.Name())
		}

		if isBaseResource(info.Name()) {
			// We already imported base resources above.
			continue
		}

		count, err := r.ImportResources(ctx, info.Name(), true)
		if err != nil {
			return errors.Wrapf(err, "cannot import %q resources", info.Name())
		}
		remainingCounts[info.Name()] = count
		s.UpdateText(fmt.Sprintf("(%d / %d) Importing %s...", i, len(grs), info.Name()))
	}
	total = 0
	for _, count := range remainingCounts {
		total += count
	}

	s.Success(importRemainingMsg + fmt.Sprintf("%d resources imported! 📥", total))
	//////////////////////////////////////////

	// At this stage, all the resources are imported, but Claims/Composites and Managed resources are paused.
	// In the finalization step, we will unpause Claims and Composites but not Managed resources (i.e. not activate the control plane yet).
	finalizeMsg := "Finalizing import... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(finalizeMsg)
	cm := category.NewAPICategoryModifier(im.dynamicClient, im.discoveryClient)
	_, err = cm.ModifyResources(ctx, "composite", func(u *unstructured.Unstructured) error {
		xpmeta.RemoveAnnotations(u, "crossplane.io/paused")
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "cannot unpause composites")
	}

	_, err = cm.ModifyResources(ctx, "claim", func(u *unstructured.Unstructured) error {
		xpmeta.RemoveAnnotations(u, "crossplane.io/paused")
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "cannot unpause claims")
	}
	s.Success(finalizeMsg + "Done! 🎉")

	if im.options.UnpauseAfterImport {
		unpauseMsg := "Unpausing managed resources ... "
		s, _ := upterm.CheckmarkSuccessSpinner.Start(unpauseMsg)
		_, err = cm.ModifyResources(ctx, "managed", func(u *unstructured.Unstructured) error {
			xpmeta.RemoveAnnotations(u, "crossplane.io/paused")
			return nil
		})
		if err != nil {
			return errors.Wrap(err, "cannot unpause managed resources")
		}
		s.Success(unpauseMsg + "Done! ▶️")
	}
	//////////////////////////////////////////

	pterm.Println("\nSuccessfully imported control plane state!")
	return nil
}

func (im *ControlPlaneStateImporter) PreflightChecks(ctx context.Context) []error {
	// Read Crossplane information from the target control plane.
	observed, err := crossplane.CollectInfo(ctx, im.appsClient)
	if err != nil {
		return []error{errors.Wrap(err, "Cannot get Crossplane info")}
	}

	// If the state archive not already unarchived, do it now, so that we can read the export metadata.
	if im.fs == nil {
		im.fs = &afero.Afero{Fs: afero.NewMemMapFs()}

		if err := im.unarchive(ctx, *im.fs); err != nil {
			return []error{errors.Wrap(err, "Cannot unarchive export archive")}
		}
	}
	b, err := im.fs.ReadFile("export.yaml")
	if err != nil {
		return []error{errors.Wrap(err, "Cannot read export metadata")}
	}
	em := &v1alpha1.ExportMeta{}
	if err = yaml.Unmarshal(b, em); err != nil {
		return []error{errors.Wrap(err, "Cannot unmarshal export metadata")}
	}

	var errs []error

	if observed.Version != em.Crossplane.Version {
		errs = append(errs, errors.Errorf("Crossplane version %q does not match exported version %q", observed.Version, em.Crossplane.Version))
	}

	for _, ff := range em.Crossplane.FeatureFlags {
		if !contains(observed.FeatureFlags, ff) {
			errs = append(errs, errors.Errorf("Feature flag %q was set in the exported control plane but is not set in the target control plane for import.", ff))
		}
	}

	return errs
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func (im *ControlPlaneStateImporter) unarchive(ctx context.Context, fs afero.Afero) error {
	g, err := os.Open(im.options.InputArchive)
	if err != nil {
		return errors.Wrap(err, "cannot open input archive")
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

	format := archiver.Tar{}

	handler := func(ctx context.Context, f archiver.File) error {
		if f.IsDir() {
			if err = fs.Mkdir(f.NameInArchive, 0700); err != nil {
				return errors.Wrapf(err, "cannot create directory %q", f.Name())
			}
			return nil
		}

		nf, err := fs.OpenFile(f.NameInArchive, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return errors.Wrapf(err, "cannot create file %q", f.Name())
		}
		defer func() {
			_ = nf.Close()
		}()

		b, err := f.Open()
		if err != nil {
			return errors.Wrapf(err, "cannot open file %q", f.Name())
		}
		defer func() {
			_ = b.Close()
		}()
		_, err = io.Copy(nf, b)
		if err != nil {
			return err
		}

		return nil
	}

	return format.Extract(ctx, ur, nil, handler)
}

func isBaseResource(gr string) bool {
	for _, k := range baseResources {
		if k == gr {
			return true
		}
	}
	return false
}

func (im *ControlPlaneStateImporter) waitForConditions(ctx context.Context, sp *pterm.SpinnerPrinter, gk schema.GroupKind, conditions []xpv1.ConditionType) error {
	rm, err := im.resourceMapper.RESTMapping(gk)
	if err != nil {
		return errors.Wrapf(err, "cannot get REST mapping for %q", gk)
	}

	success := false
	timeout := 10 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		resourceList, err := im.dynamicClient.Resource(rm.Resource).List(ctx, v1.ListOptions{})
		if err != nil {
			pterm.Printf("cannot list packages with error: %v\n", err)
			return
		}
		total := len(resourceList.Items)
		unmet := 0
		for _, r := range resourceList.Items {
			paved := fieldpath.Pave(r.Object)
			status := xpv1.ConditionedStatus{}
			if err = paved.GetValueInto("status", &status); err != nil && !fieldpath.IsNotFound(err) {
				pterm.Printf("cannot get status for %q %q with error: %v\n", gk.Kind, r.GetName(), err)
				return
			}

			for _, c := range conditions {
				if status.GetCondition(c).Status != corev1.ConditionTrue {
					unmet++
					break // At least one condition is not met, so we should break and not count the same resource multiple times.
				}
			}
		}
		if unmet > 0 {
			sp.UpdateText(fmt.Sprintf("(%d / %d) Waiting for %s to be %s...", total-unmet, total, rm.Resource.GroupResource().String(), printConditions(conditions)))
			return
		}

		success = true
		cancel()
	}, 5*time.Second)

	if !success {
		return errors.Errorf("timeout waiting for conditions %q to be satisfied for all %q", printConditions(conditions), gk.Kind)
	}

	return nil
}

func printConditions(conditions []xpv1.ConditionType) string {
	switch len(conditions) {
	case 0:
		return ""
	case 1:
		return string(conditions[0])
	case 2:
		return fmt.Sprintf("%s and %s", conditions[0], conditions[1])
	default:
		cs := make([]string, len(conditions))
		for i, c := range conditions {
			cs[i] = string(c)
		}
		return fmt.Sprintf("%s, and %s", strings.Join(cs[:len(cs)-1], ", "), cs[len(cs)-1])
	}
}
