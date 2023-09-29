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

package controlplane

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const errNoConfigurationFound = "no configuration associated to this control plane"

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd gets a single control plane in an account on Upbound.
type getCmd struct {
	Name string `arg:"" required:"" help:"Name of control plane." predictor:"ctps"`
}

// Run executes the get command.
func (c *getCmd) Run(printer upterm.ObjectPrinter, cc *cp.Client, upCtx *upbound.Context) error {
	if upCtx.Profile.IsSpace() {
		return fmt.Errorf("get is not supported for space profile %q", upCtx.ProfileName)
	}

	ctp, err := cc.Get(context.Background(), upCtx.Account, c.Name)
	if err != nil {
		return err
	}
	// All Upbound managed control planes in an account should be associated to a configuration.
	if ctp.ControlPlane.Configuration == EmptyControlPlaneConfiguration() {
		return errors.New(errNoConfigurationFound)
	}

	return printer.Print(*ctp, cloudFieldNames, extractCloudFields)
}

// EmptyControlPlaneConfiguration returns an empty ControlPlaneConfiguration with default values.
func EmptyControlPlaneConfiguration() cp.ControlPlaneConfiguration {
	configuration := cp.ControlPlaneConfiguration{}
	configuration.Status = cp.ConfigurationInstallationQueued
	return configuration
}
