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

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/cloud"
)

const (
	errDeleteCPToken = "failed to delete control plane token"
)

// deleteCmd deletes a control plane token on Upbound Cloud.
type deleteCmd struct {
	ID uuid.UUID `arg:"" required:"" help:"ID of token."`
}

// Run executes the delete command.
func (c *deleteCmd) Run(kong *kong.Context, client *tokens.Client, cloudCtx *cloud.Context) error {
	return errors.Wrap(client.Delete(context.Background(), c.ID), errDeleteCPToken)
}
