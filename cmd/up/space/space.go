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

	"github.com/upbound/up/cmd/up/space/billing"
)

const mxeChart = "spaces"

// Cmd contains commands for interacting with spaces.
type Cmd struct {
	Billing billing.Cmd `cmd:""`

	Install   installCmd   `cmd:"" maturity:"alpha" help:"Install Upbound."`
	Uninstall uninstallCmd `cmd:"" maturity:"alpha" help:"Uninstall Upbound."`
	Upgrade   upgradeCmd   `cmd:"" maturity:"alpha" help:"Upgrade Upbound."`
}

type commonParams struct {
	Repo *url.URL `hidden:"" env:"UPBOUND_REPO" default:"us-west1-docker.pkg.dev/orchestration-build/upbound-environments" help:"Set repo for Upbound."`

	Registry *url.URL `hidden:"" env:"UPBOUND_REGISTRY_ENDPOINT" default:"https://us-west1-docker.pkg.dev" help:"Set registry for authentication."`
}
