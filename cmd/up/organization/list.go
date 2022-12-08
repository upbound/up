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
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd lists organizations on Upbound.
type listCmd struct{}

var fieldNames = []string{"NAME", "ROLE"}

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	orgs, err := oc.List(context.Background())
	if err != nil {
		return err
	}
	if len(orgs) == 0 {
		p.Printfln("No organizations found.")
		return nil
	}
	return printer.Print(orgs, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	o := obj.(organizations.Organization)
	return []string{o.Name, string(o.Role)}
}
