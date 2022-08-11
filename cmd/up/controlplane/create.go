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

	"github.com/pterm/pterm"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	op "github.com/upbound/up-sdk-go/service/oldplanes"
	"github.com/upbound/up/internal/upbound"
)

// CreateCmd creates a control plane on Upbound.
type CreateCmd struct {
	Name string `arg:"" required:"" help:"Name of control plane."`

	Description string `short:"d" help:"Description for control plane."`
}

// Run executes the create command.
func (c *CreateCmd) Run(experimental bool, p pterm.TextPrinter, cc *cp.Client, oc *op.Client, upCtx *upbound.Context) error {
	if experimental {
		if _, err := cc.Create(context.Background(), upCtx.Account, &cp.ControlPlaneCreateParameters{
			Name:        c.Name,
			Description: c.Description,
		}); err != nil {
			return err
		}
	} else {
		if _, err := oc.Create(context.Background(), &op.ControlPlaneCreateParameters{
			Account:     upCtx.Account,
			Name:        c.Name,
			Description: c.Description,
		}); err != nil {
			return err
		}
	}

	p.Printfln("%s created.", c.Name)
	return nil
}
