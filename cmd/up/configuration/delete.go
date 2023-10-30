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
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/configurations"
	"github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *deleteCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *deleteCmd) AfterApply(cc *configurations.Client, cpc *controlplanes.Client, p pterm.TextPrinter, upCtx *upbound.Context) error {
	if c.Force {
		return nil
	}
	// Deleting a configuration can orphan any control planes that have it deployed.
	// While the API will eventually return a 400 status, we can show the user
	// which control planes are using the configuration.
	cfg, err := cc.Get(context.Background(), upCtx.Account, c.Name)
	if err != nil {
		return err
	}
	cpList, err := cpc.List(context.Background(), upCtx.Account, common.ListOption(controlplanes.WithConfiguration(cfg.ID)))
	if err != nil {
		return err
	}
	if cpList.Count > 0 {
		deployedOn := []string{}
		for _, cp := range cpList.ControlPlanes {
			deployedOn = append(deployedOn, cp.ControlPlane.Name)
		}
		return fmt.Errorf("this configuration is still in use by control plane(s): %v", deployedOn)
	}

	confirm, err := c.prompter.Prompt("Are you sure you want to delete this configuration? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting configuration %s. This cannot be undone,", c.Name)
		return nil
	}

	return fmt.Errorf("operation canceled")
}

// deleteCmd deletes a single root configuration by name on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	Name string `arg:"" required:"" name:"The name of the configuration." predictor:"configs"`

	Force bool `help:"Force deletion of the configuration." default:"false"`
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, cc *configurations.Client, upCtx *upbound.Context) error {
	if err := cc.Delete(ctx, upCtx.Account, c.Name); err != nil {
		return err
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
