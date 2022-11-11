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

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd deletes a robot token on Upbound.
type getCmd struct {
	RobotName string `arg:"" required:"" help:"Name of robot."`
	TokenName string `arg:"" required:"" help:"Name of token."`
}

// Run executes the get robot token command.
func (c *getCmd) Run(p pterm.TextPrinter, pt *pterm.TablePrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, tc *tokens.Client, upCtx *upbound.Context) error { //nolint:gocyclo
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

	// We pick the first robot account with this name, though there
	// may be more than one. If a user wants to see all of the tokens
	// for robots with the same name, they can use the list commands
	var rid *uuid.UUID
	for _, r := range rs {
		if r.Name == c.RobotName {
			// Pin range variable so that we can take address.
			r := r
			rid = &r.ID
			break
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

	// We pick the first token with this name, though there may be more
	// than one. If a user wants to see all of the tokens with the same name
	// they can use the list command.
	var theToken *common.DataSet
	for _, t := range ts.DataSet {
		if fmt.Sprint(t.AttributeSet["name"]) == c.TokenName {
			// Pin range variable so that we can take address.
			t := t
			theToken = &t
			break
		}
	}
	if theToken == nil {
		return errors.Errorf(errFindTokenFmt, c.TokenName, c.RobotName, upCtx.Account)
	}
	tList := []common.DataSet{*theToken}
	return printTokens(tList, pt)
}
