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
	"fmt"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/exporter"
)

type exportCmd struct {
}

func (c *exportCmd) Run(ctx context.Context, migCtx *migration.Context) error {
	fmt.Println("Exporting ...")

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
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	e := exporter.NewControlPlaneStateExporter(crdClient, dynamicClient, mapper, exporter.Options{
		OutputArchive: "xp-state.tar.gz",
		// TODO(turkenh): Pass these options from the CLI.
		ExcludedNamespaces: []string{"kube-system", "kube-public", "kube-node-lease", "local-path-storage"},
		IncludedResources:  []string{"namespaces", "configmaps", "secrets"}, // + all Crossplane resources
	})
	if err = e.Export(ctx); err != nil {
		return err
	}

	fmt.Println("Export complete!")
	return nil
}