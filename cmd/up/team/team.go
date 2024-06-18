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
	"github.com/posener/complete"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/teams"

	"github.com/upbound/up/internal/upbound"
)

const (
	errUserAccount = "teams are not currently supported for user accounts"
)

// AfterApply constructs and binds a teams client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	kongCtx.Bind(accounts.NewClient(cfg))
	kongCtx.Bind(organizations.NewClient(cfg))
	kongCtx.Bind(teams.NewClient(cfg))
	return nil
}

func PredictTeams() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return nil
		}

		ac := accounts.NewClient(cfg)
		if ac == nil {
			return nil
		}

		oc := organizations.NewClient(cfg)
		if oc == nil {
			return nil
		}

		account, err := ac.Get(context.Background(), upCtx.Account)
		if err != nil {
			return nil
		}
		if account.Account.Type != accounts.AccountOrganization {
			return nil
		}
		ts, err := oc.ListTeams(context.Background(), account.Organization.ID)
		if err != nil {
			return nil
		}
		if len(ts) == 0 {
			return nil
		}
		data := make([]string, len(ts))
		for i, t := range ts {
			data[i] = t.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with teams.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a team."`
	Delete deleteCmd `cmd:"" help:"Delete a team."`
	List   listCmd   `cmd:"" help:"List teams."`
	Get    getCmd    `cmd:"" help:"Get a team."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
