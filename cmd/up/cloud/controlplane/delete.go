package controlplane

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
)

// DeleteCmd deletes a control plane on Upbound Cloud.
type DeleteCmd struct {
	// TODO(hasheddan): consider using name instead of ID
	ID uuid.UUID `arg:"" required:"" help:"ID of control plane."`
}

// Run executes the delete command.
func (c *DeleteCmd) Run(kong *kong.Context, client *cp.Client) error {
	return client.Delete(context.Background(), c.ID)
}
