package config

import (
	"github.com/alecthomas/kong"
	"github.com/upbound/up/internal/upbound"
)

// Cmd contains commands for Upbound Profiles.
type Cmd struct {
	Current currentCmd `cmd:"" group:"config" help:"Get current Upbound Profile."`
	List    listCmd    `cmd:"" group:"config" help:"List Upbound Profiles."`

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
