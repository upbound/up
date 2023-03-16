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

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// createCmd sends out an invitation to a user to join an organization.
type createCmd struct {
	OrgName    string                                    `arg:"" required:"" help:"Name of the organization."`
	Email      string                                    `arg:"" required:"" help:"Email address of the user to invite."`
	Permission organizations.OrganizationPermissionGroup `short:"p" enum:"member,owner" default:"member" help:"Role of the user to invite (owner or member)."`
}

// Run executes the invite command.
func (c *createCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgID, err := oc.GetOrgID(context.Background(), c.OrgName)
	if err != nil {
		return err
	}

	if err = oc.CreateInvite(context.Background(), orgID, &organizations.OrganizationInviteCreateParameters{
		Email:      c.Email,
		Permission: c.Permission,
	}); err != nil {
		return err
	}

	p.Printfln("%s invited", c.Email)
	return nil
}
