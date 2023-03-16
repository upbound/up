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

package invite

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// deleteCmd deletes an invitation to a user to join an organization.
type deleteCmd struct {
	prompter input.Prompter

	OrgName     string `arg:"" required:"" help:"Name of the organization."`
	InviteEmail string `arg:"" required:"" help:"Email of the invitation to delete."`

	Force bool `help:"Force deletion of the invite." default:"false"`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *deleteCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *deleteCmd) AfterApply(p pterm.TextPrinter) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to delete this invite? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting invite %s. This cannot be undone.", c.InviteEmail)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// Run executes the create command.
func (c *deleteCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgID, err := oc.GetOrgID(context.Background(), c.OrgName)
	if err != nil {
		return err
	}
	currentInvites, err := oc.ListInvites(context.Background(), orgID)
	if err != nil {
		return err
	}

	for _, invite := range currentInvites {
		if invite.Email == c.InviteEmail {
			if err := oc.DeleteInvite(context.Background(), orgID, invite.ID); err != nil {
				return err
			}
			p.Printfln("Invitation %d deleted", invite.ID)
			return nil
		}
	}

	return fmt.Errorf("no invitation found for email %s", c.InviteEmail)
}
