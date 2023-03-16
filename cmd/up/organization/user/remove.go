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

package user

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// removeCmd removes a user from an organization.
// Ideally it would take the username or email. Today that information
// isn't available via the API, so the user ID is required.
type removeCmd struct {
	prompter input.Prompter

	OrgName string `arg:"" required:"" help:"Name of the organization."`
	UserID  uint   `arg:"" required:"" help:"ID of the user to remove."`

	Force bool `help:"Force removal of the member." default:"false"`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *removeCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *removeCmd) AfterApply(p pterm.TextPrinter) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to remove this member? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Removing member %d. This cannot be undone.", c.UserID)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// Run executes the remove-member command.
func (c *removeCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgID, err := oc.GetOrgID(context.Background(), c.OrgName)
	if err != nil {
		return err
	}
	if err = oc.RemoveMember(context.Background(), orgID, c.UserID); err != nil {
		return err
	}

	p.Printfln("User %d removed from %s", c.UserID, c.OrgName)
	return nil
}
