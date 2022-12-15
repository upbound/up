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

package user

import (
	"context"

	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// removeMemberCmd removes a user from an organization.
type removeMemberCmd struct {
	OrgID  uint `arg:"" required:"" help:"ID of the organization."`
	UserID uint `arg:"" required:"" help:"ID of the user to remove."`
}

// Run executes the invite command.
func (c *removeMemberCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, oc *organizations.Client, upCtx *upbound.Context) error {
	if err := oc.RemoveMember(context.Background(), c.OrgID, c.UserID); err != nil {
		return err
	}

	p.Printfln("User %d removed", c.UserID)
	return nil
}
