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

package organization

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up-sdk-go/service/organizations"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds an organizations client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	cfg, err := upCtx.BuildSDKConfig(upCtx.Profile.Session)
	if err != nil {
		return err
	}
	config.Helpers.OrgsClient = organizations.NewClient(cfg)
	kongCtx.Bind(upCtx)
	kongCtx.Bind(config.Helpers.OrgsClient)
	return nil
}

func PredictOrgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) (prediction []string) {
		c, err := upbound.MakeClients()
		oc := c.OrgClient
		if oc == nil || err != nil {
			return nil
		}

		orgs, err := oc.List(context.Background())
		if err != nil {
			return nil
		}

		if len(orgs) == 0 {
			return nil
		}

		data := make([]string, len(orgs))
		for i, o := range orgs {
			data[i] = o.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with organizations.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create an organization."`
	Delete deleteCmd `cmd:"" help:"Delete an organization." predictor:"orgs"`
	List   listCmd   `cmd:"" help:"List organizations."`
	Get    getCmd    `cmd:"" help:"Get an organization."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
