/*
Copyright 2021 Upbound Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controlplane

import (
	"context"

	"github.com/google/uuid"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
)

// DeleteCmd deletes a control plane on Upbound Cloud.
type DeleteCmd struct {
	// TODO(hasheddan): consider using name instead of ID
	ID uuid.UUID `arg:"" required:"" help:"ID of control plane."`
}

// Run executes the delete command.
func (c *DeleteCmd) Run(client *cp.Client) error {
	return client.Delete(context.Background(), c.ID)
}
