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

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/teams"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

var fieldNames = []string{"TEAMS"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd lists all teams a specific robot belongs to.
type listCmd struct {
	RobotName string `arg:"" required:"" help:"Name of robot."`
}

// Run executes the get robot command to get all team memberships for a specific robot.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, tc *teams.Client, upCtx *upbound.Context) error { //nolint:gocyclo
	a, err := ac.Get(ctx, upCtx.Account)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	rs, err := oc.ListRobots(ctx, a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}

	var rid *uuid.UUID
	for _, r := range rs {
		if r.Name == c.RobotName {
			r := r
			rid = &r.ID
			break
		}
	}
	if rid == nil {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}

	robot, err := rc.Get(ctx, *rid)
	if err != nil {
		return err
	}

	teamsRs, ok := robot.RelationshipSet["teams"].(map[string]any)
	if !ok {
		return errors.New("unexpected format for team relationships")
	}

	data, ok := teamsRs["data"].([]any)
	if !ok {
		return errors.New("unexpected format for team data")
	}

	teamIDs := []uuid.UUID{}

	for _, d := range data {
		team, ok := d.(map[string]any)
		if !ok {
			return errors.New("unexpected format for team relationship")
		}

		idStr, ok := team["id"].(string)
		if !ok {
			return errors.New("unexpected format for team ID")
		}

		id, err := uuid.Parse(idStr)
		if err != nil {
			return err
		}
		teamIDs = append(teamIDs, id)
	}

	teamInfos := make([]teams.TeamResponse, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		team, err := tc.Get(ctx, teamID)
		if err != nil {
			return err
		}
		teamInfos = append(teamInfos, *team)
	}

	return printer.Print(teamInfos, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	team := obj.(teams.TeamResponse)
	return []string{team.Name}
}
