package config

import (
	"github.com/pkg/errors"

	"github.com/upbound/up/internal/upbound"
)

type useCmd struct {
	Name string `arg:"" required:"" help:"Name of the Profile to use."`
}

// Run executes the Use command.
func (c *useCmd) Run(upCtx *upbound.Context) error {
	if err := upCtx.Cfg.SetDefaultUpboundProfile(c.Name); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateConfig)
}
