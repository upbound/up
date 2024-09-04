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

package connector

import (
	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *uninstallCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	if c.ClusterName == "" {
		c.ClusterName = c.Namespace
	}
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	mgr, err := helm.NewManager(kubeconfig,
		connectorName,
		mcpRepoURL,
		helm.WithNamespace(c.InstallationNamespace),
		helm.IsOCI(),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.mgr = mgr
	return nil
}

// uninstallCmd uninstalls UXP.
type uninstallCmd struct {
	mgr install.Manager

	ClusterName           string `help:"Name of the cluster connecting to the control plane. If not provided, the namespace argument value will be used."`
	Namespace             string `arg:"" required:"" help:"Namespace in the control plane where the claims of the cluster will be stored."`
	Kubeconfig            string `type:"existingfile" help:"Override the default kubeconfig path."`
	InstallationNamespace string `short:"n" env:"MCP_CONNECTOR_NAMESPACE" default:"kube-system" help:"Kubernetes namespace for MCP Connector. Default is kube-system."`
}

// Run executes the uninstall command.
func (c *uninstallCmd) Run(p pterm.TextPrinter) error {
	if err := c.mgr.Uninstall(); err != nil {
		return err
	}
	p.Printfln("MCP Connector uninstalled")
	return nil
}
