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
	"errors"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// removeCmd removes a user from an organization.
// The user can be specified by username or email address.
// If the user has been invited (but not yet joined) the invite is removed.
// If the user is a member of the organization, the user is removed.
type removeCmd struct {
	prompter input.Prompter

	OrgName string `arg:"" required:"" help:"Name of the organization."`
	User    string `arg:"" required:"" help:"Username or email of the user to remove."`

	Force bool `help:"Force removal of the member." default:"false"`
}

const (
	errUserNotFound = "user not found"
)

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
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// Run executes the remove command.
func (c *removeCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgID, err := oc.GetOrgID(context.Background(), c.OrgName)
	if err != nil {
		return err
	}

	// First try to remove an invite.
	inviteID, err := findInviteID(oc, orgID, c.User)
	if err == nil {
		if err = oc.DeleteInvite(context.Background(), orgID, inviteID); err != nil {
			return err
		}

		p.Printfln("Invite for %s removed from %s", c.User, c.OrgName)
		return nil
	}

	// If no invite was found, try to remove a member.
	userID, err := findUserID(oc, orgID, c.User)
	if err == nil {
		if err = oc.RemoveMember(context.Background(), orgID, userID); err != nil {
			return err
		}
		p.Printfln("Member %s removed from %s", c.User, c.OrgName)
		return nil
	}

	return errors.New(errUserNotFound)
}

// findInviteID returns the invite ID for the given email address, if it exists.
func findInviteID(oc *organizations.Client, orgID uint, email string) (uint, error) {
	invites, err := oc.ListInvites(context.Background(), orgID)
	if err != nil {
		return 0, err
	}
	for _, invite := range invites {
		if invite.Email == email {
			return invite.ID, nil
		}
	}
	return 0, errors.New(errUserNotFound)
}

// findUserID returns the user ID for the given username or email address, if it exists.
func findUserID(oc *organizations.Client, orgID uint, username string) (uint, error) {
	users, err := oc.ListMembers(context.Background(), orgID)
	if err != nil {
		return 0, err
	}
	for _, user := range users {
		if user.User.Username == username || user.User.Email == username {
			return user.User.ID, nil
		}
	}
	return 0, errors.New(errUserNotFound)
}
