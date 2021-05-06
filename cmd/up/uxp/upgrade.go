package uxp

import (
	"github.com/upbound/up/internal/uxp"
	"github.com/upbound/up/internal/uxp/installers/helm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(uxpCtx *uxp.Context) error {
	installer, err := helm.NewInstaller(uxpCtx.Kubeconfig,
		helm.WithNamespace(uxpCtx.Namespace),
		helm.AllowUnstableVersions(c.Unstable))
	if err != nil {
		return err
	}
	c.installer = installer
	return nil
}

// upgradeCmd upgrades UXP.
type upgradeCmd struct {
	installer uxp.Installer

	Version string `arg:"" optional:"" help:"UXP version to upgrade to."`

	Unstable bool `help:"Allow installing unstable UXP versions."`
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run(uxpCtx *uxp.Context) error {
	return c.installer.Upgrade(c.Version)
}
