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

package migration

import (
	"context"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/exporter"
)

type exportCmd struct {
	Output string `short:"o" help:"Output archive path." default:"xp-state.tar.gz"`

	IncludedResources []string `help:"Included additional resources." default:"namespaces,configmaps,secrets"` // + all Crossplane resources
	ExcludedResources []string `help:"Resources that should not be exported."`                                 // default: none

	IncludedNamespaces []string `help:"Namespaces that should be exported."` // default: none
	ExcludedNamespaces []string `help:"Namespaces that should not be exported." default:"kube-system,kube-public,kube-node-lease,local-path-storage"`

	PauseBeforeExport bool `help:"Pause all managed resources before exporting." default:"false"`
}

func (c *exportCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	cfg := migCtx.Kubeconfig

	crdClient, err := apiextensionsclientset.NewForConfig(cfg)
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	appsClient, err := appsv1.NewForConfig(cfg)
	if err != nil {
		return err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	e := exporter.NewControlPlaneStateExporter(crdClient, dynamicClient, discoveryClient, appsClient, mapper, exporter.Options{
		OutputArchive: c.Output,

		ExcludedNamespaces: c.ExcludedNamespaces,
		IncludedResources:  c.IncludedResources,

		PauseBeforeExport: c.PauseBeforeExport,
	})
	if err = e.Export(ctx); err != nil {
		return err
	}
	return nil
}
