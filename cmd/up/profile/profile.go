// Copyright 2022 Upbound Inc
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

package profile

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/profile/config"
	"github.com/upbound/up/internal/upbound"
)

// Cmd contains commands for Upbound Profiles.
type Cmd struct {
	Config  config.Cmd `cmd:"" group:"profile" help:"Interact with the current Upbound Profile's config."`
	Current currentCmd `cmd:"" group:"profile" help:"Get current Upbound Profile."`
	List    listCmd    `cmd:"" group:"profile" help:"List Upbound Profiles."`
	Use     useCmd     `cmd:"" group:"profile" help:"Set the default Upbound Profile to the given Profile."`
	View    viewCmd    `cmd:"" group:"profile" help:"View the Upbound Profile settings across profiles."`

	Flags upbound.Flags `embed:""`
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}

	kongCtx.Bind(upCtx)
	return nil
}
