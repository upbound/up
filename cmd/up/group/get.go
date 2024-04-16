// Copyright 2024 Upbound Inc
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

package group

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// getCmd gets a specific group in a space.
type getCmd struct {
	Name string `arg:"" required:"" help:"Name of group."`
}

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))

	return nil
}

// Run executes the list command.
func (c *getCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, upCtx *upbound.Context, p pterm.TextPrinter) error { // nolint:gocyclo
	// get context
	_, currentProfile, _, err := upCtx.Cfg.GetCurrentContext(ctx)
	if err != nil {
		return err
	}
	if currentProfile == nil {
		return errors.New(profile.NoSpacesContextMsg)
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

	// list groups
	var ns corev1.Namespace
	if err := cl.Get(ctx, types.NamespacedName{Name: c.Name}, &ns); err != nil {
		return err
	}

	// only print the group if it is a registered group
	if _, ok := ns.Labels[spacesv1beta1.ControlPlaneGroupLabelKey]; !ok {
		return fmt.Errorf("namespace %q is not a group", c.Name)
	}

	return printer.Print(ns, fieldNames, extractGroupFields)
}
