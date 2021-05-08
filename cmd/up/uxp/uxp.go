package uxp

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/uxp"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(ctx *kong.Context) error {
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}
	ctx.Bind(&uxp.Context{
		Kubeconfig: kubeconfig,
		Namespace:  c.Namespace,
	})
	return nil
}

// Cmd contains commands for managing UXP.
type Cmd struct {
	Install   installCmd   `cmd:"" group:"uxp" help:"Install UXP."`
	Uninstall uninstallCmd `cmd:"" group:"uxp" help:"Uninstall UXP."`
	Upgrade   upgradeCmd   `cmd:"" group:"uxp" help:"Upgrade UXP."`
	Connect   connectCmd   `cmd:"" group:"uxp" help:"Connect UXP to Upbound Cloud."`

	Kubeconfig string `env:"KUBECONFIG" type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" default:"upbound-system" help:"Kubernetes namespace for UXP."`
}
