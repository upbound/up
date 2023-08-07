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
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
)

const spacesChart = "spaces"

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	kongCtx.Bind(&install.Context{
		Kubeconfig: kubeconfig,
	})
	return nil
}

// Cmd contains commands for interacting with spaces.
type Cmd struct {
	Billing    billing.Cmd `cmd:""`
	Kubeconfig string      `type:"existingfile" help:"Override default kubeconfig path."`

	Init    initCmd    `cmd:"" help:"Initialize an Upbound Spaces deployment."`
	Destroy destroyCmd `cmd:"" help:"Remove the Upbound Spaces deployment."`
	Upgrade upgradeCmd `cmd:"" help:"Upgrade the Upbound Spaces deployment."`
}

type commonParams struct {
	Repo *url.URL `hidden:"" env:"UPBOUND_REPO" default:"us-west1-docker.pkg.dev/orchestration-build/upbound-environments" help:"Set repo for Upbound."`

	Registry *url.URL `hidden:"" env:"UPBOUND_REGISTRY_ENDPOINT" default:"https://us-west1-docker.pkg.dev" help:"Set registry for authentication."`
}
