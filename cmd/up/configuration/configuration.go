// Copyright 2023 Upbound Inc
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

package configuration

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up-sdk-go/service/gitsources"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a configurations client to any subcommands
// such as "list" or "get" that have Run() methods that receive it.
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
	kongCtx.Bind(configurations.NewClient(cfg))
	kongCtx.Bind(gitsources.NewClient(cfg))
	return nil
}

func PredictConfigurations() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return nil
		}

		cp := configurations.NewClient(cfg)
		if cp == nil {
			return nil
		}

		configs, err := cp.List(context.Background(), upCtx.Account)
		if err != nil {
			return nil
		}

		if len(configs.Configurations) == 0 {
			return nil
		}

		data := make([]string, len(configs.Configurations))
		for i, config := range configs.Configurations {
			data[i] = *config.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with root configurations.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a configuration."`
	List   listCmd   `cmd:"" help:"List root configurations for the account."`
	Get    getCmd    `cmd:"" help:"Get a single configuration for the account."`

	Flags upbound.Flags `embed:""`
}
