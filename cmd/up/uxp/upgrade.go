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
	"io"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

const (
	errParseUpgradeParameters = "unable to parse upgrade parameters"
)

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(insCtx *install.Context) error {
	repo := uxpRepoURL
	if c.Unstable {
		repo = uxpUnstableRepoURL
	}
	ins, err := helm.NewManager(insCtx.Kubeconfig,
		chartName,
		repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithChart(c.Bundle),
		helm.WithAlternateChart(alternateChartName),
		helm.RollbackOnError(c.Rollback),
		helm.Force(c.Force))
	if err != nil {
		return err
	}
	c.mgr = ins
	base := map[string]interface{}{}
	if c.File != nil {
		defer c.File.Close() //nolint:errcheck,gosec
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &base); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := c.File.Close(); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}
	c.parser = helm.NewParser(base, c.Set)
	return nil
}

// upgradeCmd upgrades UXP.
type upgradeCmd struct {
	mgr    install.Manager
	parser install.ParameterParser

	Version string `arg:"" optional:"" help:"UXP version to upgrade to."`

	Rollback bool `help:"Rollback to previously installed version on failed upgrade."`
	Force    bool `help:"Force upgrade even if versions are incompatible."`
	Unstable bool `help:"Allow installing unstable versions."`

	install.CommonParams
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run(insCtx *install.Context) error {
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseUpgradeParameters)
	}
	return c.mgr.Upgrade(c.Version, params)
}
