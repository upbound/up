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

package team

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/teams"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

const (
	errMultipleTeamFmt = "found multiple teams with name %s in %s"
	errFindTeamFmt     = "could not find team %s in %s"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *deleteCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *deleteCmd) AfterApply(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to delete this team? This cannot be undone. [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting team %s/%s. ", upCtx.Account, c.Name)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// deleteCmd deletes a team on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	Name string `arg:"" required:"" help:"Name of team." predictor:"teams"`

	Force bool `help:"Force delete team even if conflicts exist." default:"false"`
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, tc *teams.Client, upCtx *upbound.Context) error { //nolint:gocyclo
	a, err := ac.Get(ctx, upCtx.Account)
	if err != nil {
		return err
	}

	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	rs, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindTeamFmt, c.Name, upCtx.Account)
	}

	var id *uuid.UUID
	for _, r := range rs {
		if r.Name == c.Name {
			if id != nil && !c.Force {
				return errors.Errorf(errMultipleTeamFmt, c.Name, upCtx.Account)
			}
			r := r
			id = &r.ID
		}
	}

	if id == nil {
		return errors.Errorf(errFindTeamFmt, c.Name, upCtx.Account)
	}

	if err := tc.Delete(ctx, *id); err != nil {
		return err
	}
	p.Printfln("%s/%s deleted", upCtx.Account, c.Name)
	return nil
}
