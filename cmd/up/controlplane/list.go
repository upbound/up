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
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// listCmd list control planes in an account on Upbound.
type listCmd struct {
	AllGroups bool `short:"A" default:"false" help:"List control planes across all groups."`
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context, client client.Client) error {
	// todo(redbackthomson): Add support for `-A`

	var l spacesv1beta1.ControlPlaneList
	if err := client.List(ctx, &l); err != nil {
		return errors.Wrap(err, "error getting control planes")
	}

	if len(l.Items) == 0 {
		p.Println("No control planes found")
		return nil
	}

	return tabularPrint(l, printer, upCtx)
}
