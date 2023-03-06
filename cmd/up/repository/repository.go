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

package repository

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/repositories"

	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a repositories client to any subcommands
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
	kongCtx.Bind(repositories.NewClient(cfg))
	return nil
}

func PredictRepos() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return nil
		}

		rc := repositories.NewClient(cfg)
		if rc == nil {
			return nil
		}

		repos, err := rc.List(context.Background(), upCtx.Account, common.WithSize(maxItems))
		if err != nil {
			return nil
		}

		if len(repos.Repositories) == 0 {
			return nil
		}

		data := make([]string, len(repos.Repositories))
		for i, o := range repos.Repositories {
			data[i] = o.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with repositories.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a repository."`
	Delete deleteCmd `cmd:"" help:"Delete a repository."`
	List   listCmd   `cmd:"" help:"List repositories for the account."`
	Get    getCmd    `cmd:"" help:"Get a repository for the account."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}
