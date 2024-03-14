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

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd gets a single robot in an account on Upbound.
type getCmd struct {
	Name string `arg:"" required:"" help:"Name of robot." predictor:"robots"`
}

// Run executes the get robot command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, ac *accounts.Client, oc *organizations.Client, upCtx *upbound.Context) error {
	a, err := ac.Get(ctx, upCtx.Account)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}

	// The get command accepts a name, but the get API call takes an ID
	// Therefore we get all robots and find the one the user requested
	// The API doesn't guarantee uniqueness, but we just print the first
	// one we find. If a user wants to list all of them, they can use
	// the list command.
	rs, err := oc.ListRobots(ctx, a.Organization.ID)
	if err != nil {
		return err
	}

	for _, r := range rs {
		if r.Name == c.Name {
			return printer.Print(r, fieldNames, extractFields)
		}
	}
	return errors.New("no robot named \"" + c.Name + "\"")
}
