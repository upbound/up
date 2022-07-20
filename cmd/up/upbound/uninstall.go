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

package upbound

import (
	"net/url"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(insCtx *install.Context) error {
	// NOTE(hasheddan): we do not specify credentials or that chart is from an
	// OCI image as they are inconsequential at uninstall time.
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		upboundChart,
		&url.URL{},
		helm.WithNamespace(insCtx.Namespace))
	if err != nil {
		return err
	}
	c.mgr = mgr
	return nil
}

// uninstallCmd uninstalls Upbound.
type uninstallCmd struct {
	mgr install.Manager
}

// Run executes the uninstall command.
func (c *uninstallCmd) Run(insCtx *install.Context) error {
	return c.mgr.Uninstall()
}
