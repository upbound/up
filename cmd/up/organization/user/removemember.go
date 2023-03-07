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

// removeMemberCmd removes a user from an organization.
type removeMemberCmd struct {
	prompter input.Prompter

	OrgID  uint `arg:"" required:"" help:"ID of the organization."`
	UserID uint `arg:"" required:"" help:"ID of the user to remove."`

	Force bool `help:"Force removal of the member." default:"false"`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *removeMemberCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *removeMemberCmd) AfterApply(p pterm.TextPrinter) error {
	if c.Force {
		return nil
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to remove this member? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Removing member %s. This cannot be undone.", c.UserID)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// Run executes the invite command.
func (c *removeMemberCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	if err := oc.RemoveMember(context.Background(), c.OrgID, c.UserID); err != nil {
		return err
	}

	p.Printfln("User %d removed", c.UserID)
	return nil
}
