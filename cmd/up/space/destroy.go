// Copyright 2023 Upbound Inc
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

package space

import (
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upterm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *destroyCmd) AfterApply(insCtx *install.Context) error {
	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		spacesChart,
		c.Repo,
		helm.WithNamespace(ns),
		helm.IsOCI())
	if err != nil {
		return err
	}
	c.mgr = mgr
	return nil
}

// destroyCmd uninstalls Upbound.
type destroyCmd struct {
	mgr install.Manager

	commonParams
}

// Run executes the uninstall command.
func (c *destroyCmd) Run(insCtx *install.Context) error {
	return c.mgr.Uninstall()
}
