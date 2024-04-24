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

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd gets a single control plane in an account on Upbound.
type getCmd struct {
	Name  string `arg:"" required:"" help:"Name of control plane." predictor:"ctps"`
	Group string `short:"g" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current profile."`
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context) error {
	_, currentProfile, currentCtp, err := upCtx.Cfg.GetCurrentContext(ctx)
	if err != nil {
		return err
	}
	if currentProfile == nil {
		return errors.New(profile.NoSpacesContextMsg)
	}
	if currentCtp.Namespace == "" {
		return errors.New(profile.NoGroupMsg)
	}
	if currentCtp.Name != "" {
		return errors.New("Cannot get control plane from inside a control plane, use `up ctx ..` to switch to a group level.")
	}

	// create client
	restConfig, _, err := currentProfile.GetSpaceRestConfig()
	if err != nil {
		return err
	}
	cl, err := ctrlclient.New(restConfig, ctrlclient.Options{})
	if err != nil {
		return err
	}

	ns := currentCtp.Namespace
	if c.Group != "" {
		ns = c.Group
	}

	ctp := &spacesv1beta1.ControlPlane{}
	if err := cl.Get(ctx, types.NamespacedName{Name: c.Name, Namespace: ns}, ctp); err != nil {
		if kerrors.IsNotFound(err) {
			p.Printfln("Control plane %s not found", c.Name)
			return nil
		}
		return err
	}
	return printer.Print(*ctp, spacefieldNames, extractSpaceFields)
}

// EmptyControlPlaneConfiguration returns an empty ControlPlaneConfiguration with default values.
func EmptyControlPlaneConfiguration() cp.ControlPlaneConfiguration {
	configuration := cp.ControlPlaneConfiguration{}
	configuration.Status = cp.ConfigurationInstallationQueued
	return configuration
}
