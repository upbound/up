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
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/repositorypermission"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// listCmd lists repository permissions for a team on Upbound.
type listCmd struct {
	TeamName string `arg:"" required:"" help:"Name of the team."`
}

// fieldNames for the list output.
var fieldNames = []string{"TEAM", "REPOSITORY", "PERMISSION", "CREATED", "UPDATED"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rpc *repositorypermission.Client, upCtx *upbound.Context) error { //nolint:gocyclo
	// Get account details
	a, err := ac.Get(ctx, upCtx.Account)
	if err != nil {
		return errors.Wrap(err, "cannot get accounts")
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New("user account is not an organization")
	}

	// Get the list of teams
	ts, err := oc.ListTeams(ctx, a.Organization.ID)
	if err != nil {
		return errors.Wrap(err, "cannot list teams")
	}

	// Create a map from team IDs to team names
	teamIDToName := make(map[uuid.UUID]string)
	for _, t := range ts {
		teamIDToName[t.ID] = t.Name
	}

	// Find the team ID for the specified team name
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
		return fmt.Errorf("could not find team %s in account %s", c.TeamName, upCtx.Account)
	}

	// List repository permissions for the team
	resp, err := rpc.List(ctx, upCtx.Account, teamID)
	if err != nil {
		return errors.Wrap(err, "cannot list permissions")
	}
	if len(resp.Permissions) == 0 {
		p.Printfln("No repository permissions found for team %s in account %s", c.TeamName, upCtx.Account)
		return nil
	}

	return printer.Print(resp.Permissions, fieldNames, func(obj any) []string {
		return extractFields(obj, teamIDToName)
	})
}

// extractFields extracts the fields for printing.
func extractFields(obj any, teamIDToName map[uuid.UUID]string) []string {
	p := obj.(repositorypermission.Permission)

	updated := "n/a"
	if p.UpdatedAt != nil {
		updated = duration.HumanDuration(time.Since(*p.UpdatedAt))
	}

	teamName := teamIDToName[p.TeamID]

	return []string{teamName, p.RepositoryName, string(p.Privilege), p.CreatedAt.Format(time.RFC3339), updated}
}
