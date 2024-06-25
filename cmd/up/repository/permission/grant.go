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
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/repositorypermission"
	"github.com/upbound/up/internal/upbound"
)

// grantCmd grant repositorypermission for an team on Upbound.
type grantCmd struct {
	TeamName       string `arg:"" required:"" help:"Name of team."`
	RepositoryName string `arg:"" required:"" help:"Name of repository."`
	Permission     string `arg:"" required:"" help:"Permission type (admin, read, write, view)."`
}

// Validate validates the grantCmd struct.
func (c *grantCmd) Validate() error {
	switch repositorypermission.PermissionType(c.Permission) {
	case repositorypermission.PermissionAdmin, repositorypermission.PermissionRead, repositorypermission.PermissionWrite, repositorypermission.PermissionView:
		return nil
	default:
		return fmt.Errorf("invalid permission type %q: must be one of [admin, read, write, view]", c.Permission)
	}
}

// Run executes the create command.
func (c *grantCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rpc *repositorypermission.Client, upCtx *upbound.Context) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("permission validation failed for team %q in account %q: %w", c.TeamName, upCtx.Account, err)
	}

	a, err := ac.Get(ctx, upCtx.Account)
	if err != nil {
		return errors.Wrap(err, "cannot get accounts")
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New("user account is not an organization")
	}

	ts, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return errors.Wrap(err, "cannot list teams")
	}

	// Find the team with the specified name
	var teamID uuid.UUID
	teamFound := false
	for _, t := range ts {
		if t.Name == c.TeamName {
			teamID = t.ID
			teamFound = true
			break
		}
	}
	if !teamFound {
		return fmt.Errorf("could not find team %q in account %q", c.TeamName, upCtx.Account)
	}

	if err := rpc.Create(ctx, upCtx.Account, teamID, repositorypermission.CreatePermission{
		Repository: c.RepositoryName,
		Permission: repositorypermission.RepositoryPermission{
			Permission: repositorypermission.PermissionType(c.Permission),
		},
	}); err != nil {
		return errors.Wrap(err, "cannot grant permission")
	}
	p.Printfln("Permission %q granted to team %q for repository %q in account %q", c.Permission, c.TeamName, c.RepositoryName, upCtx.Account)
	return nil
}
