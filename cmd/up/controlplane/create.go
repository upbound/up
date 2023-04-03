// Copyright 2021 Upbound Inc
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

package controlplane

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up-sdk-go/service/controlplanes"
)

// createCmd creates a control plane on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of control plane."`

	ConfigurationName string `required:"" help:"The name of the Configuration."`
	Description       string `short:"d" help:"Description for control plane."`
}

// Run executes the create command.
func (c *createCmd) Run(p pterm.TextPrinter, a *accounts.AccountResponse, cfg *up.Config) error {
	// Get the UUID from the Configuration name, if it exists.
	configPkg, err := configurations.NewClient(cfg).Get(context.Background(), a.Account.Name, c.ConfigurationName)
	if err != nil {
		return err
	}

	if _, err := controlplanes.NewClient(cfg).Create(context.Background(), a.Account.Name, &controlplanes.ControlPlaneCreateParameters{
		Name:            c.Name,
		Description:     c.Description,
		ConfigurationID: configPkg.ID,
	}); err != nil {
		return err
	}

	p.Printfln("%s created", c.Name)
	return nil
}
