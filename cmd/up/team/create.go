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

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/teams"

	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a team on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of Team."`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, tc *teams.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Account)
	if err != nil {
		return err
	}

	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	if _, err := tc.Create(ctx, &teams.TeamCreateParameters{
		Name:           c.Name,
		OrganizationID: a.Organization.ID,
	},
	); err != nil {
		return err
	}
	p.Printfln("%s/%s created", upCtx.Account, c.Name)
	return nil
}
