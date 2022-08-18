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

package token

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/internal/upbound"
)

// deleteCmd deletes a robot token on Upbound.
type deleteCmd struct {
	RobotName string `arg:"" required:"" help:"Name of robot."`
	TokenName string `arg:"" required:"" help:"Name of token."`

	Force bool `hidden:"" help:"Force delete token even if conflicts exist."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, tc *tokens.Client, upCtx *upbound.Context) error { //nolint:gocyclo
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
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}
	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one robot exists in the specified account
	// with the provided name. Logic should be simplified when the API is
	// updated.
	var rid *uuid.UUID
	for _, r := range rs {
		if r.Name == c.RobotName {
			if rid != nil {
				return errors.Errorf(errMultipleRobotFmt, c.RobotName, upCtx.Account)
			}
			// Pin range variable so that we can take address.
			r := r
			rid = &r.ID
		}
	}
	if rid == nil {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}

	ts, err := rc.ListTokens(context.Background(), *rid)
	if err != nil {
		return err
	}
	if len(ts.DataSet) == 0 {
		return errors.Errorf(errFindTokenFmt, c.TokenName, c.RobotName, upCtx.Account)
	}

	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one token exists for the specified robot in
	// the specified account with the provided name. Logic should be simplified
	// when the API is updated.
	var tid *uuid.UUID
	for _, t := range ts.DataSet {
		if fmt.Sprint(t.AttributeSet["name"]) == c.TokenName {
			if tid != nil && !c.Force {
				return errors.Errorf(errMultipleTokenFmt, c.TokenName, c.RobotName, upCtx.Account)
			}
			// Pin range variable so that we can take address.
			t := t
			tid = &t.ID
		}
	}
	if tid == nil {
		return errors.Errorf(errFindTokenFmt, c.TokenName, c.RobotName, upCtx.Account)
	}

	if err := tc.Delete(context.Background(), *tid); err != nil {
		return err
	}
	p.Printfln("%s/%s/%s deleted", upCtx.Account, c.RobotName, c.TokenName)
	return nil
}
