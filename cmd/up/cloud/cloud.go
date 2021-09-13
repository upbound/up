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
	"net/url"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(ctx *kong.Context) error {
	conf, src, err := config.Extract()
	if err != nil {
		return err
	}
	ctx.Bind(&upbound.Context{
		Profile:  c.Profile,
		Account:  c.Account,
		Endpoint: c.Endpoint,
		Cfg:      conf,
		CfgSrc:   src,
	})
	return nil
}

// Cmd contains commands for interacting with Upbound Cloud.
type Cmd struct {
	Login  loginCmd  `cmd:"" group:"cloud" help:"Login to Upbound Cloud."`
	Logout logoutCmd `cmd:"" group:"cloud" help:"Logout from Upbound Cloud."`

	ControlPlane controlPlaneCmd `cmd:"" name:"controlplane" aliases:"ctp" group:"cloud" help:"Interact with control planes."`

	Endpoint *url.URL `env:"UP_ENDPOINT" default:"https://api.upbound.io" help:"Endpoint used for Upbound API."`
	Profile  string   `env:"UP_PROFILE" help:"Profile used to execute command."`
	Account  string   `short:"a" env:"UP_ACCOUNT" help:"Account used to execute command."`
}
