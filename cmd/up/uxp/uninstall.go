package uxp

import (
	"github.com/upbound/up/internal/uxp"
	"github.com/upbound/up/internal/uxp/installers/helm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(uxpCtx *uxp.Context) error {
	installer, err := helm.NewInstaller(uxpCtx.Kubeconfig,
		helm.WithNamespace(uxpCtx.Namespace))
	if err != nil {
		return err
	}
	c.installer = installer
	return nil
}

// uninstallCmd uninstalls UXP.
type uninstallCmd struct {
	installer uxp.Installer
}

// Run executes the uninstall command.
func (c *uninstallCmd) Run(uxpCtx *uxp.Context) error {
	return c.installer.Uninstall()
}
