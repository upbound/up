package cloud

import (
	"github.com/alecthomas/kong"
	"github.com/pkg/errors"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/cmd/up/cloud/controlplane"
	"github.com/upbound/up/internal/cloud"
	"github.com/upbound/up/internal/config"
)

const (
	errNoOrg = "no organization was specified and a default could not be found"
)

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c controlPlaneCmd) AfterApply(ctx *kong.Context, cloudCtx *cloud.Context) error {
	// TODO(hasheddan): the majority of this logic can be used generically
	// across cloud commands when others are implemented.
	var profile config.Profile
	var name string
	var err error
	if cloudCtx.Profile == "" {
		name, profile, err = cloudCtx.Cfg.GetDefaultCloudProfile()
		if err != nil {
			return err
		}
		cloudCtx.Profile = name
		cloudCtx.ID = profile.ID
	} else {
		profile, err = cloudCtx.Cfg.GetCloudProfile(cloudCtx.Profile)
		if err != nil {
			return err
		}
	}
	// If org has not already been set, use the profile default.
	if cloudCtx.Org == "" {
		cloudCtx.Org = profile.Org
	}
	// If no org is set in profile, return an error.
	if cloudCtx.Org == "" {
		return errors.New(errNoOrg)
	}
	cfg, err := cloud.BuildSDKConfig(profile.Session, cloudCtx.Endpoint)
	if err != nil {
		return err
	}
	ctx.Bind(cp.NewClient(cfg))
	ctx.Bind(tokens.NewClient(cfg))
	return nil
}

// controlPlaneCmd contains commands for interacting with control planes.
type controlPlaneCmd struct {
	Attach controlplane.AttachCmd `cmd:"" help:"Attach a self-hosted control plane."`
	Create controlplane.CreateCmd `cmd:"" help:"Create a hosted control plane."`
	Delete controlplane.DeleteCmd `cmd:"" help:"Delete a control plane."`
}
