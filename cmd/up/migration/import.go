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
	"fmt"
	"regexp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/pkg/migration"
	"github.com/upbound/up/pkg/migration/importer"
)

var (
	// Matches https://00.000.000.0.nip.io/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp1/k8s
	newControlPlanePathRE = regexp.MustCompile(`^(?P<base>.+)/apis/spaces.upbound.io/(?P<version>v[^/]+)/namespaces/(?P<namespace>[^/]+)/controlplanes/(?P<controlplane>[^/]+)/k8s$`)
	// Matches https://spaces-foo.upboundrocks.cloud/v1/controlplanes/acmeco/default/ctp/k8s
	oldControlPlanePathRE = regexp.MustCompile(`^(?P<base>.+)/v1/control[pP]lanes/(?P<account>[^/]+)/(?P<namespace>[^/]+)/(?P<controlplane>[^/]+)/k8s$`)
)

type importCmd struct {
	prompter input.Prompter
	Yes      bool `help:"When set to true, automatically accepts any confirmation prompts that may appear during the import process." default:"false"`

	Input string `short:"i" help:"Specifies the file path of the archive to be imported. The default path is 'xp-state.tar.gz'." default:"xp-state.tar.gz"`

	UnpauseAfterImport bool `help:"When set to true, automatically unpauses all managed resources that were paused during the import process. This helps in resuming normal operations post-import. Defaults to false, requiring manual unpausing of resources if needed." default:"false"`
}

func (c *importCmd) Help() string {
	return `
Usage:
    migration import [options]

The 'import' command imports a control plane state from an archive file into an Upbound managed control plane.

By default, all managed resources will be paused during the import process for possible manual inspection/validation.
You can use the --unpause-after-import flag to automatically unpause all managed resources after the import process completes.

Examples:
    migration import --input=my-export.tar.gz
        Imports the control plane state from 'my-export.tar.gz'.

    migration import --unpause-after-import
        Imports and automatically unpauses managed resources after import.
`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *importCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

func (c *importCmd) Run(ctx context.Context, migCtx *migration.Context) error { //nolint:gocyclo // Just a lot of error handling.
	cfg := migCtx.Kubeconfig

	if !isMCP(cfg.Host) {
		return errors.New("not a managed control plane, import not supported!")
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

	appsClient, err := appsv1.NewForConfig(cfg)
	if err != nil {
		return err
	}

	i := importer.NewControlPlaneStateImporter(dynamicClient, discoveryClient, appsClient, mapper, importer.Options{
		InputArchive: c.Input,

		UnpauseAfterImport: c.UnpauseAfterImport,
	})

	errs := i.PreflightChecks(ctx)
	if len(errs) > 0 {
		fmt.Println("Preflight checks failed:")
		for _, err := range errs {
			fmt.Println("- " + err.Error())
		}
		if !c.Yes {
			pterm.Println() // Blank line
			confirm := pterm.DefaultInteractiveConfirm
			confirm.DefaultText = "Do you still want to proceed?"
			confirm.DefaultValue = false
			result, _ := confirm.Show()
			pterm.Println() // Blank line
			if !result {
				pterm.Error.Println("Preflight checks must pass in order to proceed with the import.")
				return nil
			}
		}
	}

	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	pterm.Println("Importing control plane state...")
	migration.DefaultSpinner = &spinner{upterm.CheckmarkSuccessSpinner}

	if err = i.Import(ctx); err != nil {
		return err
	}
	pterm.Println("\nfully imported control plane state!")

	return nil
}

func isMCP(host string) bool {
	return newControlPlanePathRE.MatchString(host) || oldControlPlanePathRE.MatchString(host)
}
