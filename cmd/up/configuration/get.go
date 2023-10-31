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

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// getCmd gets a single root configuration in an account on Upbound.
type getCmd struct {
	Name string `arg:"" required:"" name:"The name of the configuration." predictor:"configs"`
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, cc *configurations.Client, upCtx *upbound.Context) error {
	cfg, err := cc.Get(ctx, upCtx.Account, c.Name)
	if err != nil {
		return err
	}

	// To match the list output (fieldNames), we convert to the list response.
	cfgList := configurations.ConfigurationListResponse{
		Configurations: []configurations.ConfigurationResponse{*cfg},
	}
	return printer.Print(cfgList.Configurations, fieldNames, extractFields)
}
