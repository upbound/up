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
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/controlplane/kubeconfig"
	"github.com/upbound/up/cmd/up/controlplane/token"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upbound"
)

const (
	errNoAccount = "no account was specified and a default could not be found"
)

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c *controlPlaneCmd) AfterApply(kongCtx *kong.Context) error {
	// TODO(hasheddan): the majority of this logic can be used generically
	// across upbound commands when others are implemented.
	src := config.NewFSSource()
	if err := src.Initialize(); err != nil {
		return err
	}
	conf, err := config.Extract(src)
	if err != nil {
		return err
	}
	upCtx := &upbound.Context{
		Profile:  c.Profile,
		Account:  c.Account,
		Endpoint: c.Endpoint,
		Cfg:      conf,
		CfgSrc:   src,
	}
	var profile config.Profile
	var name string
	if upCtx.Profile == "" {
		name, profile, err = upCtx.Cfg.GetDefaultUpboundProfile()
		if err != nil {
			return err
		}
		upCtx.Profile = name
		upCtx.ID = profile.ID
	} else {
		profile, err = upCtx.Cfg.GetUpboundProfile(upCtx.Profile)
		if err != nil {
			return err
		}
	}
	// If account has not already been set, use the profile default.
	if upCtx.Account == "" {
		upCtx.Account = profile.Account
	}
	// If no account is set in profile, return an error.
	if upCtx.Account == "" {
		return errors.New(errNoAccount)
	}
	cfg, err := upbound.BuildSDKConfig(profile.Session, upCtx.Endpoint)
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
	Attach controlplane.AttachCmd `cmd:"" help:"Attach a self-hosted control plane."`
	Create controlplane.CreateCmd `cmd:"" help:"Create a hosted control plane."`
	Delete controlplane.DeleteCmd `cmd:"" help:"Delete a control plane."`
	List   controlplane.ListCmd   `cmd:"" help:"List control planes for the account."`

	Kubeconfig kubeconfig.Cmd `cmd:"" name:"kubeconfig" help:"Manage control plane kubeconfig data."`
	Token      token.Cmd      `cmd:"" name:"token" help:"Interact with control plane tokens."`

	// Common Upbound API configuration
	Endpoint *url.URL `env:"UP_ENDPOINT" default:"https://api.upbound.io" help:"Endpoint used for Upbound API."`
	Profile  string   `env:"UP_PROFILE" help:"Profile used to execute command."`
	Account  string   `short:"a" env:"UP_ACCOUNT" help:"Account used to execute command."`
}
