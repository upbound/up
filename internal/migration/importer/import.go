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
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	fmt.Println("Importing base resources")
	for _, gr := range baseResources {
		if err = r.ImportResources(ctx, schema.ParseGroupResource(gr)); err != nil {
			return errors.Wrapf(err, "cannot import %q resources", gr)
		}
	}

	fmt.Println("Waiting for all XRDs to be established")
	if err = i.waitForConditions(ctx, schema.GroupKind{Group: "apiextensions.crossplane.io", Kind: "CompositeResourceDefinition"}, []xpv1.ConditionType{"Established"}); err != nil {
		return errors.Wrap(err, "there are unhealthy CompositeResourceDefinitions")
	}

	fmt.Println("Waiting for all packages to be installed and healthy")
	for _, k := range []schema.GroupKind{
		{Group: "pkg.crossplane.io", Kind: "Provider"},
		{Group: "pkg.crossplane.io", Kind: "Function"},
		{Group: "pkg.crossplane.io", Kind: "Configuration"},
	} {
		if err = i.waitForConditions(ctx, k, []xpv1.ConditionType{"Installed", "Healthy"}); err != nil {
			return errors.Wrapf(err, "there are unhealthy %qs", k.Kind)
		}
	}

	// Reset the resource mapper to make sure all CRDs introduced by packages
	// or XRDs are available.
	i.resourceMapper.Reset()

	fmt.Println("Importing all other resources")
	grs, err := fs.ReadDir("/")
	if err != nil {
		return errors.Wrap(err, "cannot list group resources")
	}
	for _, gr := range grs {
		if !gr.IsDir() {
			return errors.Errorf("unexpected file %q in root directory of exported state", gr.Name())
		}

		if isBaseResource(gr.Name()) {
			// We already imported base resources above.
			continue
		}

		if err = r.ImportResources(ctx, schema.ParseGroupResource(gr.Name())); err != nil {
			return errors.Wrapf(err, "cannot import %q resources", gr.Name())
		}
	}

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

func (i *ControlPlaneStateImporter) waitForConditions(ctx context.Context, gk schema.GroupKind, conditions []xpv1.ConditionType) error {
	rm, err := i.resourceMapper.RESTMapping(gk)
	if err != nil {
		return errors.Wrapf(err, "cannot get REST mapping for %q", gk)
	}

	success := false
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		resourceList, err := i.dynamicClient.Resource(rm.Resource).List(ctx, v1.ListOptions{})
		if err != nil {
			fmt.Printf("cannot list packages with error: %v\n", err)
			return
		}
		for _, r := range resourceList.Items {
			paved := fieldpath.Pave(r.Object)
			status := xpv1.ConditionedStatus{}
			if err = paved.GetValueInto("status", &status); err != nil {
				fmt.Printf("cannot get status for %q %q with error: %v\n", gk.Kind, r.GetName(), err)
				return
			}

			for _, c := range conditions {
				if status.GetCondition(c).Status != corev1.ConditionTrue {
					fmt.Printf("%q %q is not %s yet\n", gk.Kind, r.GetName(), c)
					return
				}
			}
		}
		success = true
		cancel()
		return
	}, 5*time.Second)

	if !success {
		return errors.Errorf("timeout waiting for conditions %q to be satisfied for all %q", conditions, gk.Kind)
	}

	return nil
}
