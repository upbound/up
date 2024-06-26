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

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/repositorypermission"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

// revokeCmd revoke the repository permission for a team on Upbound.
type revokeCmd struct {
	prompter input.Prompter

	TeamName       string `arg:"" required:"" help:"Name of team."`
	RepositoryName string `arg:"" required:"" help:"Name of repository."`
	Force          bool   `help:"Force the revoke of the repository permission even if conflicts exist." default:"false"`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *revokeCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *revokeCmd) AfterApply(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt(fmt.Sprintf("Are you sure you want to revoke the permission for team %q in repository %q? This cannot be undone [y/n]", c.TeamName, c.RepositoryName), false)
	if err != nil {
		return errors.Wrap(err, "error with revoke prompt")
	}

	if input.InputYes(confirm) {
		p.Printfln("Revoking repository permission for team %q in repository %q", c.TeamName, c.RepositoryName)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// Run executes the delete command.
func (c *revokeCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rpc *repositorypermission.Client, upCtx *upbound.Context) error {
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

	if err := rpc.Delete(ctx, upCtx.Account, teamID, repositorypermission.PermissionIdentifier{
		Repository: c.RepositoryName,
	}); err != nil {
		return errors.Wrap(err, "cannot revoke permission")
	}

	p.Printfln("Repository permission for team %q in repository %q revoked", c.TeamName, c.RepositoryName)
	return nil
}
