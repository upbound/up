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
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

var listInvitesFieldNames = []string{"ID", "EMAIL", "PERMISSION"}

// AfterApply sets default values in command after assignment and validation.
func (c *listInvitesCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listInvitesCmd lists invites in an organization.
type listInvitesCmd struct {
	OrgName string `arg:"" required:"" help:"Name of the organization."`
}

// Run executes the list command.
func (c *listInvitesCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgID, err := oc.GetOrgID(context.Background(), c.OrgName)
	if err != nil {
		return err
	}
	resp, err := oc.ListInvites(context.Background(), orgID)
	if err != nil {
		return err
	}

	return printer.Print(resp, listInvitesFieldNames, extractInviteFields)
}

func extractInviteFields(obj any) []string {
	m := obj.(organizations.Invite)
	return []string{strconv.Itoa(int(m.ID)), m.Email, string(m.Permission)}
}
