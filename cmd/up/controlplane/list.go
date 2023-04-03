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

	"github.com/upbound/up-sdk-go"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/upterm"
)

const (
	maxItems = 100
)

const (
	notAvailable = "n/a"
)

var fieldNames = []string{"NAME", "ID", "STATUS", "DEPLOYED CONFIGURATION", "CONFIGURATION STATUS"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd list control planes in an account on Upbound.
type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, a *accounts.AccountResponse, cfg *up.Config) error {
	// TODO(hasheddan): we currently just max out single page size, but we
	// may opt to support limiting page size and iterating through pages via
	// flags in the future.
	cpList, err := controlplanes.NewClient(cfg).List(context.Background(), a.Account.Name, common.WithSize(maxItems))
	if err != nil {
		return err
	}
	if len(cpList.ControlPlanes) == 0 {
		p.Printfln("No control planes found in %s", a.Account.Name)
		return nil
	}
	return printer.Print(cpList.ControlPlanes, fieldNames, extractFields)
}

func extractFields(obj any) []string {
	c := obj.(controlplanes.ControlPlaneResponse)
	var cfgName string
	var cfgStatus string
	// All Upbound managed control planes in an account should be associated to a configuration.
	// However, we should still list all control planes and indicate where this isn't the case.
	if c.ControlPlane.Configuration.Name != nil && c.ControlPlane.Configuration != EmptyControlPlaneConfiguration() {
		cfgName = *c.ControlPlane.Configuration.Name
		cfgStatus = string(c.ControlPlane.Configuration.Status)
	} else {
		cfgName, cfgStatus = notAvailable, notAvailable
	}
	return []string{c.ControlPlane.Name, c.ControlPlane.ID.String(), string(c.Status), cfgName, cfgStatus}
}
