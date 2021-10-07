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

package enterprise

import (
	"io"
	"net/url"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

const (
	errParseUpgradeParameters = "unable to parse upgrade parameters"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *upgradeCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(insCtx *install.Context) error {
	id, err := c.prompter.Prompt("License ID", false)
	if err != nil {
		return err
	}
	token, err := c.prompter.Prompt("Token", true)
	if err != nil {
		return err
	}
	ins, err := helm.NewManager(insCtx.Kubeconfig,
		enterpriseChart,
		c.Repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithBasicAuth(id, token),
		helm.IsOCI(),
		helm.WithChart(c.Bundle),
		helm.RollbackOnError(c.Rollback))
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

// upgradeCmd upgrades enterprise.
type upgradeCmd struct {
	mgr      install.Manager
	parser   install.ParameterParser
	prompter input.Prompter

	// NOTE(hasheddan): version is currently required for upgrade with OCI image
	// as latest strategy is undetermined.
	Version string `arg:"" help:"Enterprise version to upgrade to."`

	Rollback bool     `help:"Rollback to previously installed version on failed upgrade."`
	Repo     *url.URL `hidden:"" env:"ENTERPRISE_REPO" default:"registry.upbound.io/enterprise" help:"Set repo for enterprise."`

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
