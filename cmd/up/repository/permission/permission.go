// Copyright 2024 Upbound Inc
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

package permission

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/repositorypermission"
	"github.com/upbound/up-sdk-go/service/teams"

	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a clients to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(repositorypermission.NewClient(cfg))
	kongCtx.Bind(organizations.NewClient(cfg))
	kongCtx.Bind(accounts.NewClient(cfg))
	kongCtx.Bind(teams.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing repository permissions for teams.
type Cmd struct {
	Grant  grantCmd  `cmd:"" help:"Grant repository permission for a team."`
	Revoke revokeCmd `cmd:"" help:"Revoke repository permission from a team."`
	List   listCmd   `cmd:"" help:"List all repository permissions for teams."`
}
