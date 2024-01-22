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
	"net/url"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/migration"
	"github.com/upbound/up/internal/migration/importer"
)

type importCmd struct {
	prompter input.Prompter
	Yes      bool `help:"Skip confirmation prompts."`

	Input string `short:"i" help:"Input archive path." default:"xp-state.tar.gz"`

	UnpauseAfterImport bool `help:"Unpause all managed resources after importing." default:"true"`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *importCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

func (c *importCmd) Run(ctx context.Context, migCtx *migration.Context) error { //nolint:gocyclo // Just a lot of error handling.
	cfg := migCtx.Kubeconfig

	if !isMCP(cfg) {
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
			res, err := c.prompter.Prompt("Do you still wish to proceed? [y/n]", false)
			if err != nil {
				return err
			}
			if res != "y" {
				return nil
			}
			// Print a newline to separate the prompt from the output.
			fmt.Println()
		}
	}

	if err = i.Import(ctx); err != nil {
		return err
	}

	return nil
}

func isMCP(cfg *rest.Config) bool {
	u, err := url.Parse(cfg.Host)
	if err != nil {
		return false
	}
	return (strings.HasPrefix(u.Path, "/v1/controlplanes") || strings.HasPrefix(u.Path, "/v1/controlPlanes")) && strings.HasSuffix(u.Path, "/k8s")
}
