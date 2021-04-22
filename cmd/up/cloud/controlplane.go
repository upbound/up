package cloud

import (
	"github.com/alecthomas/kong"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/cmd/up/cloud/controlplane"
	"github.com/upbound/up/internal/cloud"
)

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c controlPlaneCmd) AfterApply(ctx *kong.Context, cloudCtx *cloud.Context) error {
	cfg, err := cloud.BuildSDKConfig(cloudCtx.Session, cloudCtx.Endpoint)
	if err != nil {
		return err
	}
	ctx.Bind(cp.NewControlPlanesClient(cfg))
	return nil
}

// controlPlaneCmd contains commands for interacting with control planes.
type controlPlaneCmd struct {
	Create controlplane.CreateCmd `cmd:"" help:"Create a hosted control plane."`
	Delete controlplane.DeleteCmd `cmd:"" help:"Delete a control plane."`
}
