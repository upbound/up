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

	"github.com/google/uuid"
	"github.com/pterm/pterm"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	op "github.com/upbound/up-sdk-go/service/oldplanes"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets values in command after assignment and validation.
func (c *DeleteCmd) AfterApply(experimental bool) error {
	if !experimental {
		u, err := uuid.Parse(c.ID)
		if err != nil {
			return err
		}
		c.id = u
	}
	return nil
}

// DeleteCmd deletes a control plane on Upbound.
type DeleteCmd struct {
	id uuid.UUID

	ID string `arg:"" help:"ID of control plane. ID is name if using experimental MCP API."`
}

// Run executes the delete command.
func (c *DeleteCmd) Run(experimental bool, p pterm.TextPrinter, cc *cp.Client, oc *op.Client, upCtx *upbound.Context) error {
	if experimental {
		if err := cc.Delete(context.Background(), upCtx.Account, c.ID); err != nil {
			return err
		}
	} else {
		if err := oc.Delete(context.Background(), c.id); err != nil {
			return err
		}
	}
	p.Printfln("%s deleted", c.ID)
	return nil
}
