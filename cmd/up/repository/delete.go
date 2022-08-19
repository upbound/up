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
	"fmt"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/repositories"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/upbound"
)

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *deleteCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply accepts user input by default to confirm the delete operation.
func (c *deleteCmd) AfterApply(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if c.Force {
		return nil
	}
	confirm, err := c.prompter.Prompt("Are you sure you want to delete this repository? [y/n]", false)
	if err != nil {
		return err
	}

	if input.InputYes(confirm) {
		p.Printfln("Deleting repository %s/%s. This cannot be undone.", upCtx.Account, c.Name)
		return nil
	}

	return fmt.Errorf("operation canceled")

}

// deleteCmd deletes a repository on Upbound.
type deleteCmd struct {
	prompter input.Prompter

	Name string `arg:"" required:"" help:"Name of repository."`

	Force bool `help:"Force deletion of repository." default:"false"`
}

// Run executes the delete command.
func (c *deleteCmd) Run(p pterm.TextPrinter, rc *repositories.Client, upCtx *upbound.Context) error {
	if err := rc.Delete(context.Background(), upCtx.Account, c.Name); err != nil {
		return err
	}
	p.Printfln("%s/%s deleted", upCtx.Account, c.Name)
	return nil
}
