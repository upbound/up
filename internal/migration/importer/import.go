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

package importer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/upterm"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
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

type Options struct {
	InputArchive string // default: xp-state.tar.gz
}

type ControlPlaneStateImporter struct {
	dynamicClient  dynamic.Interface
	resourceMapper meta.ResettableRESTMapper

	options Options
}

func NewControlPlaneStateImporter(dynamicClient dynamic.Interface, mapper meta.ResettableRESTMapper, opts Options) *ControlPlaneStateImporter {
	return &ControlPlaneStateImporter{
		dynamicClient:  dynamicClient,
		resourceMapper: mapper,
		options:        opts,
	}
}

func (im *ControlPlaneStateImporter) Import(ctx context.Context) error { // nolint:gocyclo // This is the high level import command, so it's expected to be a bit complex.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	fmt.Println("Importing control plane state...")

	// Reading state from the archive
	unarchiveMsg := "Reading state from the archive... "
	s, _ := upterm.CheckmarkSuccessSpinner.Start(unarchiveMsg)
	g, err := os.Open(im.options.InputArchive)
	if err != nil {
		s.Fail(unarchiveMsg + "Failed!")
		return errors.Wrap(err, "cannot open input archive")
	}
	defer func() {
		_ = g.Close()
	}()

	ur, err := gzip.NewReader(g)
	if err != nil {
		s.Fail(unarchiveMsg + "Failed!")
		return errors.Wrap(err, "cannot decompress archive")
	}
	defer func() {
		_ = ur.Close()
	}()
	fs := afero.Afero{Fs: tarfs.New(tar.NewReader(ur))}
	s.Success(unarchiveMsg + "Done! 👀")
	//////////////////////////////////////////

	r := NewPausingResourceImporter(NewFileSystemReader(fs), NewUnstructuredResourceApplier(im.dynamicClient, im.resourceMapper))

	// Import base resources
	importBaseMsg := "Importing base resources... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(importBaseMsg + fmt.Sprintf("0 / %d", len(baseResources)))
	baseCounts := make(map[string]int, len(baseResources))
	for i, gr := range baseResources {
		count, err := r.ImportResources(ctx, gr)
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

	// Wait for all base resources to be ready
	waitXRDsMsg := "Waiting for XRDs... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(waitXRDsMsg)
	if err = im.waitForConditions(ctx, s, schema.GroupKind{Group: "apiextensions.crossplane.io", Kind: "CompositeResourceDefinition"}, []xpv1.ConditionType{"Established"}); err != nil {
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
		if err = im.waitForConditions(ctx, s, k, []xpv1.ConditionType{"Installed", "Healthy"}); err != nil {
			s.Fail(waitPkgsMsg + "Failed!")
			return errors.Wrapf(err, "there are unhealthy %qs", k.Kind)
		}
	}
	s.Success(waitPkgsMsg + "Installed and Healthy! ⏳")

	// Reset the resource mapper to make sure all CRDs introduced by packages
	// or XRDs are available.
	im.resourceMapper.Reset()

	importRemainingMsg := "Importing remaining resources... "
	s, _ = upterm.CheckmarkSuccessSpinner.Start(importRemainingMsg)
	grs, err := fs.ReadDir("/")
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

		count, err := r.ImportResources(ctx, info.Name())
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

	fmt.Println("\nSuccessfully imported control plane state!")
	return nil
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
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		resourceList, err := im.dynamicClient.Resource(rm.Resource).List(ctx, v1.ListOptions{})
		if err != nil {
			fmt.Printf("cannot list packages with error: %v\n", err)
			return
		}
		total := len(resourceList.Items)
		unmet := 0
		for _, r := range resourceList.Items {
			paved := fieldpath.Pave(r.Object)
			status := xpv1.ConditionedStatus{}
			if err = paved.GetValueInto("status", &status); err != nil {
				fmt.Printf("cannot get status for %q %q with error: %v\n", gk.Kind, r.GetName(), err)
				return
			}

			for _, c := range conditions {
				if status.GetCondition(c).Status != corev1.ConditionTrue {
					unmet++
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
