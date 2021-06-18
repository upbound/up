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

package cloud

import (
	"github.com/alecthomas/kong"
	"github.com/pkg/errors"

	"github.com/upbound/up-sdk-go/service/accounts"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/cmd/up/cloud/controlplane"
	"github.com/upbound/up/cmd/up/cloud/controlplane/token"
	"github.com/upbound/up/internal/cloud"
	"github.com/upbound/up/internal/config"
)

const (
	errNoAccount = "no account was specified and a default could not be found"
)

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c *controlPlaneCmd) AfterApply(ctx *kong.Context, cloudCtx *cloud.Context) error {
	// TODO(hasheddan): the majority of this logic can be used generically
	// across cloud commands when others are implemented.
	var profile config.Profile
	var name string
	var err error
	if cloudCtx.Profile == "" {
		name, profile, err = cloudCtx.Cfg.GetDefaultCloudProfile()
		if err != nil {
			return err
		}
		cloudCtx.Profile = name
		cloudCtx.ID = profile.ID
	} else {
		profile, err = cloudCtx.Cfg.GetCloudProfile(cloudCtx.Profile)
		if err != nil {
			return err
		}
	}
	// If account has not already been set, use the profile default.
	if cloudCtx.Account == "" {
		cloudCtx.Account = profile.Account
	}
	// If no account is set in profile, return an error.
	if cloudCtx.Account == "" {
		return errors.New(errNoAccount)
	}
	cfg, err := cloud.BuildSDKConfig(profile.Session, cloudCtx.Endpoint)
	if err != nil {
		return err
	}
	ctx.Bind(cp.NewClient(cfg))
	ctx.Bind(tokens.NewClient(cfg))
	ctx.Bind(accounts.NewClient(cfg))
	return nil
}

// controlPlaneCmd contains commands for interacting with control planes.
type controlPlaneCmd struct {
	Attach controlplane.AttachCmd `cmd:"" help:"Attach a self-hosted control plane."`
	Create controlplane.CreateCmd `cmd:"" help:"Create a hosted control plane."`
	Delete controlplane.DeleteCmd `cmd:"" help:"Delete a control plane."`
	List   controlplane.ListCmd   `cmd:"" help:"List control planes for the account."`

	Token token.Cmd `cmd:"" name:"token" help:"Interact with control plane tokens."`
}
