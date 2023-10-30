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

package robot

import (
	"context"
	"strconv"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/robots"

	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a robot on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of robot."`

	// NOTE(hasheddan): a description is required by the API, but we default to
	// ' ' to avoid forcing the user to provide one.
	Description string `default:" " help:"Description of robot."`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, ac *accounts.Client, rc *robots.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Account)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}
	if _, err := rc.Create(ctx, &robots.RobotCreateParameters{
		Attributes: robots.RobotAttributes{
			Name:        c.Name,
			Description: c.Description,
		},
		Relationships: robots.RobotRelationships{
			Owner: robots.RobotOwner{
				Data: robots.RobotOwnerData{
					Type: robots.RobotOwnerOrganization,
					ID:   strconv.FormatUint(uint64(a.Organization.ID), 10),
				},
			},
		},
	}); err != nil {
		return err
	}
	p.Printfln("%s/%s created", upCtx.Account, c.Name)
	return nil
}
