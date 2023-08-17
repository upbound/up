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

package space

import (
	"net/url"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/space/billing"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
)

const (
	spacesChart = "spaces"

	defaultRegistry = "us-west1-docker.pkg.dev/orchestration-build/upbound-environments"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for interacting with spaces.
type Cmd struct {
	kubeCmds `embed:""`
	Billing  billing.Cmd `cmd:""`
}

type kubeCmds struct {
	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`

	Init    initCmd    `cmd:"" help:"Initialize an Upbound Spaces deployment."`
	Destroy destroyCmd `cmd:"" help:"Remove the Upbound Spaces deployment."`
	Upgrade upgradeCmd `cmd:"" help:"Upgrade the Upbound Spaces deployment."`
}

type commonParams struct {
	Registry *url.URL `hidden:"" env:"UPBOUND_REGISTRY" default:"us-west1-docker.pkg.dev/orchestration-build/upbound-environments" help:"Set registry for where to pull OCI artifacts from. This is an OCI registry reference, i.e. a URL without the scheme or protocol prefix."`

	RegistryEndpoint *url.URL `hidden:"" env:"UPBOUND_REGISTRY_ENDPOINT" default:"https://us-west1-docker.pkg.dev" help:"Set registry endpoint, including scheme, for authentication."`
}

// overrideRegistry is a common function that takes the candidate registry,
// compares that against the default registry and if different overrides
// that property in the params map.
func overrideRegistry(candidate string, params map[string]any) {
	// NOTE(tnthornton) this is unfortunately brittle. If the helm chart values
	// property changes, this won't necessarily account for that.
	if candidate != defaultRegistry {
		params["registry"] = candidate
	}
}

func (c *kubeCmds) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error {
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}
	kongCtx.Bind(&install.Context{
		Kubeconfig: kubeconfig,
	})

	return nil
}
