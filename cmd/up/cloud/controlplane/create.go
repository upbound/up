package controlplane

import (
	"context"

	"github.com/alecthomas/kong"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/cloud"
)

// CreateCmd creates a hosted control plane on Upbound Cloud.
type CreateCmd struct {
	Name string ` arg:"" required:"" help:"Name of control plane."`

	Description string `short:"d" help:"Description for control plane."`
}

// Run executes the create command.
func (c *CreateCmd) Run(kong *kong.Context, client *cp.Client, cloudCtx *cloud.Context) error {
	_, err := client.Create(context.Background(), &cp.ControlPlaneCreateParameters{
		Account:     cloudCtx.Org,
		Name:        c.Name,
		Description: c.Description,
	})
	return err
}
