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
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// listCmd list control planes in an account on Upbound.
type listCmd struct {
	Group     string `short:"g" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current profile."`
	AllGroups bool   `short:"A" default:"false" help:"List control planes across all groups."`
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))

	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, upCtx *upbound.Context, p pterm.TextPrinter) error { // nolint:gocyclo
	// get context
	_, currentProfile, ctp, err := upCtx.Cfg.GetCurrentContext(ctx)
	if err != nil {
		return err
	}
	if currentProfile == nil {
		return errors.New(profile.NoSpacesContextMsg)
	}
	if ctp.Namespace == "" {
		return errors.New(profile.NoGroupMsg)
	}
	if ctp.Name != "" {
		return errors.New("Cannot list control planes from inside a control plane, use `up ctx ..` to switch to a group level.")
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

	// list control planes
	ns := ctp.Namespace
	if c.Group != "" {
		ns = c.Group
	}
	if c.AllGroups {
		ns = ""
	}
	var ctps spacesv1beta1.ControlPlaneList
	if err := cl.List(ctx, &ctps, ctrlclient.InNamespace(ns)); err != nil {
		return err
	}

	return printer.Print(ctps.Items, spacefieldNames, extractSpaceFields)
}
