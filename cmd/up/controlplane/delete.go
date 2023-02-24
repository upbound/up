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
	"github.com/upbound/up/internal/upbound"
)

// deleteCmd deletes a control plane on Upbound.
type deleteCmd struct {
	Name string `arg:"" help:"Name of control plane." predictor:"ctps"`
}

// Run executes the delete command.
func (c *deleteCmd) Run(p pterm.TextPrinter, cc *cp.Client, upCtx *upbound.Context) error {
	if err := cc.Delete(context.Background(), upCtx.Account, c.Name); err != nil {
		return err
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
