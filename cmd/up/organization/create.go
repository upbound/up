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

package organization

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
)

// createCmd creates an organization on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of organization."`
}

// Run executes the create command.
func (c *createCmd) Run(p pterm.TextPrinter, oc *organizations.Client) error {
	if err := oc.Create(context.Background(), &organizations.OrganizationCreateParameters{
		Name: c.Name,
		// NOTE(hasheddan): we default display name to the same as name.
		DisplayName: c.Name,
	}); err != nil {
		return err
	}
	p.Printfln("%s created", c.Name)
	return nil
}
