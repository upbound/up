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

package migration

import (
	"context"

	"github.com/pterm/pterm"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/exporter"
)

const secretsWarning = `Warning: A functional Crossplane control plane requires cloud provider credentials,
which are stored as Kubernetes secrets. Additionally, some managed resources provide
connection details exclusively during provisioning, and these details may not be
reconstructable post-migration. Consequently, the exported archive will incorporate
those secrets by default. To exclude secrets from the export, please use the
--excluded-resources flag.

IMPORTANT: The exported archive will contain secrets. Do you wish to proceed?`

type exportCmd struct {
	prompter input.Prompter

	Yes bool `help:"When set to true, automatically accepts any confirmation prompts that may appear during the export process." default:"false"`

	Output string `short:"o" help:"Specifies the file path where the exported archive will be saved. Defaults to 'xp-state.tar.gz'." default:"xp-state.tar.gz"`

	IncludeExtraResources []string `help:"A list of extra resource types to include in the export in \"resource.group\" format in addition to all Crossplane resources. By default, it includes namespaces, configmaps, secrets." default:"namespaces,configmaps,secrets"`
	ExcludeResources      []string `help:"A list of resource types to exclude from the export in \"resource.group\" format. No resources are excluded by default."`
	IncludeNamespaces     []string `help:"A list of specific namespaces to include in the export. If not specified, all namespaces are included by default."`
	ExcludeNamespaces     []string `help:"A list of specific namespaces to exclude from the export. Defaults to 'kube-system', 'kube-public', 'kube-node-lease', and 'local-path-storage'." default:"kube-system,kube-public,kube-node-lease,local-path-storage"`

	PauseBeforeExport bool `help:"When set to true, pauses all managed resources before starting the export process. This can help ensure a consistent state for the export. Defaults to false." default:"false"`
}

func (c *exportCmd) Help() string {
	return `
Usage:
    migration export [options]

The 'export' command is used to export the current state of a Crossplane or Universal Crossplane (xp/uxp) control plane
into an archive file. This file can then be used for migration to Upbound Managed Control Planes.

Use the available options to customize the export process, such as specifying the output file path, including or excluding
specific resources and namespaces, and deciding whether to pause managed resources before exporting.

Examples:
	migration export --pause-before-export
        Pauses all managed resources first and exports the control plane state to the default archive file named 'xp-state.tar.gz'.
    
	migration export --output=my-export.tar.gz
        Exports the control plane state to a specified file 'my-export.tar.gz'.

    migration export --include-extra-resources="customresource.group" --include-namespaces="crossplane-system,team-a,team-b"
        Exports the control plane state to a default file 'xp-state.tar.gz', with the additional resource specified and only using provided namespaces.
`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *exportCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
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

		IncludeNamespaces:     c.IncludeNamespaces,
		ExcludeNamespaces:     c.ExcludeNamespaces,
		IncludeExtraResources: c.IncludeExtraResources,
		ExcludeResources:      c.ExcludeResources,

		PauseBeforeExport: c.PauseBeforeExport,
	})

	if !c.Yes && e.IncludedExtraResource("secrets") {
		confirm := pterm.DefaultInteractiveConfirm
		confirm.DefaultText = secretsWarning
		confirm.DefaultValue = true
		result, _ := confirm.Show()
		pterm.Println() // Blank line
		if !result {
			return nil
		}
	}

	if err = e.Export(ctx); err != nil {
		return err
	}
	return nil
}
