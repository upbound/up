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

package sos

import (
	"context"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/sos/exporter"
)

type exportCmd struct {
	Output string `short:"o" help:"Specifies the file path where the exported archive will be saved. Defaults to 'xp-sos-report.tar.gz'." default:"xp-sos-report.tar.gz"`
}

func (c *exportCmd) Help() string {
	return `
	Usage:
    sos export [options]

The 'export' command is used to export the current state of a Crossplane or Universal Crossplane (XP/UXP) control plane
into an archive file.

Examples:
	sos export
        Exports the sos report to the default archive file named 'xp-sos-report.tar.gz'.
    
	sos export --output=my-export.tar.gz
	Exports the sos report to the default archive file named 'my-export.tar.gz'.
`
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
	})

	if err = e.Export(ctx); err != nil {
		return err
	}
	return nil
}
