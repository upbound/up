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
	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/space/billing"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
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
	Attach  attachCmd  `cmd:"" maturity:"alpha" help:"Connect an Upbound Space to the Upbound web console."`
	Detach  detachCmd  `cmd:"" maturity:"alpha" help:"Detach an Upbound Space from the Upbound web console."`
	Init    initCmd    `cmd:"" help:"Initialize an Upbound Spaces deployment."`
	Destroy destroyCmd `cmd:"" help:"Remove the Upbound Spaces deployment."`
	Upgrade upgradeCmd `cmd:"" help:"Upgrade the Upbound Spaces deployment."`
	List    listCmd    `cmd:"" help:"List all accessible spaces in Upbound."`

	Billing billing.Cmd `cmd:""`
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

func ensureAccount(upCtx *upbound.Context, params map[string]any) {
	// If the account name was explicitly set via helm flags, keep it.
	_, ok := params["account"]
	if ok {
		return
	}

	// Get the account from the active profile if it's set.
	if upCtx.Account != "" {
		params["account"] = upCtx.Account
		return
	}

	// Fall back to the default if we didn't find an account name
	// elsewhere. Spaces created with the default can't be attached to the
	// console, so this is not ideal, but they can be used in disconnected mode.
	params["account"] = defaultAcct
}
