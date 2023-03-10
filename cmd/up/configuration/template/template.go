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

package template

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"

	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply constructs and binds a robots client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(configurations.NewClient(cfg))
	return nil
}

// Cmd contains commands for managing configuration templates
// Today, there is only one option: List
// The creation of new configuration templates is managed by Upbound,
// not users.
type Cmd struct {
	List listCmd `cmd:"" help:"List the configuration templates."`
}

func PredictTemplates() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}
		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return nil
		}

		cc := configurations.NewClient(cfg)
		if cc == nil {
			return nil
		}

		templates, err := cc.ListTemplates(context.Background())
		if err != nil {
			return nil
		}

		if len(templates.Templates) == 0 {
			return nil
		}

		data := make([]string, len(templates.Templates))
		for i, template := range templates.Templates {
			data[i] = template.ID
		}
		return data
	})
}
