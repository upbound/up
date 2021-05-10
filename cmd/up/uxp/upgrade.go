// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package uxp

import (
	"github.com/upbound/up/internal/uxp"
	"github.com/upbound/up/internal/uxp/installers/helm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(uxpCtx *uxp.Context) error {
	installer, err := helm.NewInstaller(uxpCtx.Kubeconfig,
		helm.WithNamespace(uxpCtx.Namespace),
		helm.AllowUnstableVersions(c.Unstable),
		helm.RollbackOnError(c.Rollback))
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

	Rollback bool `help:"Rollback to previously installed version on failed upgrade."`
	Unstable bool `help:"Allow upgrading to unstable UXP versions."`
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run(uxpCtx *uxp.Context) error {
	return c.installer.Upgrade(c.Version)
}
