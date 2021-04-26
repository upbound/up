package cloud

import (
	"github.com/alecthomas/kong"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/cmd/up/cloud/controlplane"
	"github.com/upbound/up/internal/cloud"
	"github.com/upbound/up/internal/config"
)

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c controlPlaneCmd) AfterApply(ctx *kong.Context, cloudCtx *cloud.Context) error {
	var profile config.Profile
	var err error
	if cloudCtx.ID == "" {
		var id string
		id, profile, err = cloudCtx.Cfg.GetDefaultCloudProfile()
		if err != nil {
			return err
		}
		cloudCtx.ID = id
	} else {
		profile, err = cloudCtx.Cfg.GetCloudProfile(cloudCtx.ID)
		if err != nil {
			return err
		}
	}
	// If org has not already been set, use the profile default.
	if cloudCtx.Org == "" {
		cloudCtx.Org = profile.Org
	}
	// If no org is set in profile, use the ID.
	if cloudCtx.Org == "" {
		cloudCtx.Org = cloudCtx.ID
	}
	cfg, err := cloud.BuildSDKConfig(profile.Session, cloudCtx.Endpoint)
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
