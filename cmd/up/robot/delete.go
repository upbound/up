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

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"

	"github.com/upbound/up/internal/upbound"
)

const (
	errMultipleRobotFmt = "found multiple robots with name %s in %s"
	errFindRobotFmt     = "could not find robot %s in %s"
)

// deleteCmd deletes a robot on Upbound.
type deleteCmd struct {
	Name string `arg:"" required:"" help:"Name of robot."`

	Force bool `hidden:"" help:"Force delete robot even if conflicts exist."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, upCtx *upbound.Context) error { //nolint:gocyclo
	a, err := ac.Get(context.Background(), upCtx.Account)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}
	rs, err := oc.ListRobots(context.Background(), a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindRobotFmt, c.Name, upCtx.Account)
	}
	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one robot exists in the specified account
	// with the provided name. Logic should be simplified when the API is
	// updated.
	var id *uuid.UUID
	for _, r := range rs {
		if r.Name == c.Name {
			if id != nil && !c.Force {
				return errors.Errorf(errMultipleRobotFmt, c.Name, upCtx.Account)
			}
			// Pin range variable so that we can take address.
			r := r
			id = &r.ID
		}
	}

	if id == nil {
		return errors.Errorf(errFindRobotFmt, c.Name, upCtx.Account)
	}

	if err := rc.Delete(context.Background(), *id); err != nil {
		return err
	}
	p.Printfln("%s/%s deleted", upCtx.Account, c.Name)
	return nil
}
