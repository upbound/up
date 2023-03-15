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

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/common"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	maxItems = 100
)

var fieldNames = []string{"NAME", "ID", "STATUS", "DEPLOYED CONFIGURATION", "CONFIGURATION STATUS"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd list control planes in an account on Upbound.
type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, cc *cp.Client, upCtx *upbound.Context) error {
	// TODO(hasheddan): we currently just max out single page size, but we
	// may opt to support limiting page size and iterating through pages via
	// flags in the future.
	cpList, err := cc.List(context.Background(), upCtx.Account, common.WithSize(maxItems))
	if err != nil {
		return err
	}
	if len(cpList.ControlPlanes) == 0 {
		p.Printfln("No control planes found in %s", upCtx.Account)
		return nil
	}
	return printer.Print(cpList.ControlPlanes, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	c := obj.(cp.ControlPlaneResponse)
	return []string{c.ControlPlane.Name, c.ControlPlane.ID.String(), string(c.Status), *c.ControlPlane.Configuration.Name, string(c.ControlPlane.Configuration.Status)}
}
