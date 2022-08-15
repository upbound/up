package profile

import (
	"github.com/alecthomas/kong"
	"github.com/upbound/up/cmd/up/profile/config"
	"github.com/upbound/up/internal/upbound"
)

// Cmd contains commands for Upbound Profiles.
type Cmd struct {
	Config  config.Cmd `cmd:"" group:"profile" help:"Interact with the current Upbound Profile's config."`
	Current currentCmd `cmd:"" group:"profile" help:"Get current Upbound Profile."`
	List    listCmd    `cmd:"" group:"profile" help:"List Upbound Profiles."`
	Use     useCmd     `cmd:"" group:"profile" help:"Set the default Upbound Profile to the given Profile."`

	Flags upbound.Flags `embed:""`
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}

	kongCtx.Bind(upCtx)
	return nil
}
