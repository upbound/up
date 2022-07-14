// Copyright 2021 Upbound Inc
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

	"github.com/upbound/up-sdk-go/service/accounts"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/controlplane/kubeconfig"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c *controlPlaneCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	cfg, err := upCtx.BuildSDKConfig(upCtx.Profile.Session)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	kongCtx.Bind(cp.NewClient(cfg))
	kongCtx.Bind(tokens.NewClient(cfg))
	kongCtx.Bind(accounts.NewClient(cfg))
	return nil
}

// controlPlaneCmd contains commands for interacting with control planes.
type controlPlaneCmd struct {
	Create controlplane.CreateCmd `cmd:"" help:"Create a hosted control plane."`
	Delete controlplane.DeleteCmd `cmd:"" help:"Delete a control plane."`
	List   controlplane.ListCmd   `cmd:"" help:"List control planes for the account."`

	Kubeconfig kubeconfig.Cmd `cmd:"" name:"kubeconfig" help:"Manage control plane kubeconfig data."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
