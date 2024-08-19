package local

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for interacting with the dev environment.
type Cmd struct {
	Start startCmd `cmd:"" help:"Optionally build then start a local control plane."`
	Stop  stopCmd  `cmd:"" help:"Stop the current local control plane."`
}
