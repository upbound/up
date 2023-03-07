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

package token

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/uuid"
	"github.com/pterm/pterm"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a robot on Upbound.
type createCmd struct {
	RobotName string `arg:"" required:"" help:"Name of robot."`
	TokenName string `arg:"" required:"" help:"Name of token."`

	Output string `type:"path" short:"o" required:"" help:"Path to write JSON file containing access ID and token."`
}

// Run executes the create command.
func (c *createCmd) Run(p pterm.TextPrinter, ac *accounts.Client, oc *organizations.Client, rc *robots.Client, tc *tokens.Client, upCtx *upbound.Context) error { //nolint:gocyclo
	a, err := ac.Get(context.Background(), upCtx.Account)
	if err != nil {
		return err
	}
	if a.Account.Type != accounts.AccountOrganization {
		return errors.New(errUserAccount)
	}
	rs, err := oc.ListRobots(context.Background(), a.Organization.ID)
	if err != nil {
		return err
	}
	if len(rs) == 0 {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}
	// TODO(hasheddan): because this API does not guarantee name uniqueness, we
	// must guarantee that exactly one robot exists in the specified account
	// with the provided name. Logic should be simplified when the API is
	// updated.
	var id uuid.UUID
	found := false
	for _, r := range rs {
		if r.Name == c.RobotName {
			if found {
				return errors.Errorf(errMultipleRobotFmt, c.RobotName, upCtx.Account)
			}
			id = r.ID
			found = true
		}
	}
	if !found {
		return errors.Errorf(errFindRobotFmt, c.RobotName, upCtx.Account)
	}
	res, err := tc.Create(context.Background(), &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.TokenName,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerRobot,
					ID:   id,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	p.Printfln("%s/%s/%s created", upCtx.Account, c.RobotName, c.TokenName)
	if c.Output == "" {
		p.Printfln("Refusing to emit sensitive output. Please specify output location.")
		return nil
	}

	access := res.ID.String()
	token := fmt.Sprint(res.DataSet.Meta["jwt"])
	if c.Output == "-" {
		pterm.Println()
		p.Printfln(pterm.LightMagenta("Access ID: ") + access)
		p.Printfln(pterm.LightMagenta("Token: ") + token)
		return nil
	}

	f, err := os.OpenFile(filepath.Clean(c.Output), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck,gosec
	return json.NewEncoder(f).Encode(&upbound.TokenFile{
		AccessID: access,
		Token:    token,
	})
}
