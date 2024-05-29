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
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a control plane on Upbound.
type createCmd struct {
	Name  string `arg:"" required:"" help:"Name of control plane."`
	Group string `short:"g" default:"" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current context"`

	Crossplane struct {
		Version     string `default:"" help:"The version of Universal Crossplane to use. The default depends on the selected auto-upgrade channel."`
		AutoUpgrade struct {
			Channel string `default:"Stable" help:"The Crossplane auto-upgrade channel to use. Must be one of: None, Patch, Stable, Rapid" enum:"None,Patch,Stable,Rapid"`
		} `embed:""`
	} `embed:"" prefix:"crossplane-"`

	// todo(redbackthomson): Support all overrides for control planes
	// ConfigurationName *string `help:"The optional name of the Configuration."`

	SecretName string `help:"The name of the control plane's secret. Defaults to 'kubeconfig-{control plane name}'. Only applicable for Space control planes."`
}

// Validate performs custom argument validation for the create command.
func (c *createCmd) Validate() error {
	// TODO(adamwg): This validation should probably happen on the server side,
	// at which point we could remove it here.
	if c.Crossplane.Version != "" {
		if c.Crossplane.AutoUpgrade.Channel != string(spacesv1beta1.CrossplaneUpgradeNone) {
			return fmt.Errorf("upgrade channel must be %q to specify a version", string(spacesv1beta1.CrossplaneUpgradeNone))
		}
		_, err := semver.Parse(c.Crossplane.Version)
		if err != nil {
			return fmt.Errorf("invalid Crossplane version specified: %w; do not prefix the version with a 'v'", err)
		}
	}
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *createCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	if c.Group == "" {
		ns, _, err := upCtx.Kubecfg.Namespace()
		if err != nil {
			return err
		}
		c.Group = ns
	}
	return nil
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, client client.Client) error {
	ctp := &spacesv1beta1.ControlPlane{
		ObjectMeta: v1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Group,
		},
	}

	if c.Crossplane.Version != "" {
		ctp.Spec.Crossplane.Version = &c.Crossplane.Version
	}
	if c.Crossplane.AutoUpgrade.Channel != "" {
		ch := spacesv1beta1.CrossplaneUpgradeChannel(c.Crossplane.AutoUpgrade.Channel)
		ctp.Spec.Crossplane.AutoUpgradeSpec = &spacesv1beta1.CrossplaneAutoUpgradeSpec{
			Channel: &ch,
		}
	}

	if err := client.Create(ctx, ctp); err != nil {
		return errors.Wrap(err, "error creating control plane")
	}

	p.Printfln("%s created", c.Name)
	return nil
}
