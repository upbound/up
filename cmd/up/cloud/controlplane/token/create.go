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

package token

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/upbound"
)

const (
	jwtKey = "jwt"

	errNoToken = "could not identify token in response"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *createCmd) AfterApply(ctx *kong.Context) error {
	if c.Name == "" {
		c.Name = namesgenerator.GetRandomName(0)
	}
	return nil
}

// createCmd creates a control plane token on Upbound Cloud.
type createCmd struct {
	ID uuid.UUID `arg:"" name:"control-plane-ID" required:"" help:"ID of control plane."`

	Name string `help:"Name of control plane token."`
}

// Run executes the create command.
func (c *createCmd) Run(kong *kong.Context, client *tokens.Client, upCtx *upbound.Context) error {
	tRes, err := client.Create(context.Background(), &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.Name,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerControlPlane,
					ID:   c.ID,
				},
			},
		},
	})
	if err != nil {
		return err
	}
	jwt, ok := tRes.DataSet.Meta[jwtKey]
	if !ok {
		return errors.New(errNoToken)
	}
	fmt.Fprintf(kong.Stdout, "%s\n", jwt)
	return nil
}
