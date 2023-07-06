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

package main

import (
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane/cmd/crank/commands"
	"github.com/spf13/afero"
	"github.com/upbound/up-sdk-go"

	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in crossplane command after assignment and validation.
func (c *xpCmd) AfterApply(kongCtx *kong.Context) error {
	buildChild := &commands.BuildChild{
		FS: afero.NewOsFs(),
	}
	pushChild := &commands.PushChild{
		FS: afero.NewOsFs(),
	}

	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx, buildChild, pushChild)
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	c.client = cfg.Client
	return nil
}

// xpCmd embeds the crossplane kubectl plugin (crank) commands
type xpCmd struct {
	client up.Client

	Build   commands.BuildCmd   `cmd:"" help:"Build Crossplane packages."`
	Install commands.InstallCmd `cmd:"" help:"Install Crossplane packages."`
	Update  commands.UpdateCmd  `cmd:"" help:"Update Crossplane packages."`
	Push    commands.PushCmd    `cmd:"" help:"Push Crossplane packages."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
