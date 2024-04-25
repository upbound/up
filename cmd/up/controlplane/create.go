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
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a control plane on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of control plane."`

	// todo(redbackthomson): Support all overrides for control planes
	// ConfigurationName *string `help:"The optional name of the Configuration."`

	SecretName string `help:"The name of the control plane's secret. Defaults to 'kubeconfig-{control plane name}'. Only applicable for Space control planes."`
}

// AfterApply sets default values in command after assignment and validation.
func (c *createCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, client client.Client) error {
	ctp := &spacesv1beta1.ControlPlane{
		ObjectMeta: v1.ObjectMeta{
			Name: c.Name,
		},
	}

	if err := client.Create(ctx, ctp); err != nil {
		return errors.Wrap(err, "error creating control plane")
	}

	p.Printfln("%s created", c.Name)
	return nil
}
