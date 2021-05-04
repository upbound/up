package cloud

import (
	"net/url"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/cloud"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c Cmd) AfterApply(ctx *kong.Context) error {
	conf, src, err := cloud.ExtractConfig()
	if err != nil {
		return err
	}
	ctx.Bind(&cloud.Context{
		Profile:  c.Profile,
		Account:  c.Account,
		Endpoint: c.Endpoint,
		Cfg:      conf,
		CfgSrc:   src,
	})
	return nil
}

// Cmd contains commands for interacting with Upbound Cloud.
type Cmd struct {
	Login loginCmd `cmd:"" group:"cloud" help:"Login to Upbound Cloud."`

	ControlPlane controlPlaneCmd `cmd:"" name:"controlplane" aliases:"xp" group:"cloud" help:"Interact with control planes."`

	Endpoint *url.URL `env:"UP_ENDPOINT" default:"https://api.upbound.io" help:"Endpoint used for Upbound API."`
	Profile  string   `env:"UP_PROFILE" help:"Profile used to execute command."`
	Account  string   `short:"a" env:"UP_ACCOUNT" help:"Account used to execute command."`
}
