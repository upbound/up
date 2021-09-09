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
	"net/url"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
)

const (
	chartName          = "universal-crossplane"
	alternateChartName = "crossplane"
)

var (
	uxpRepoURL, _         = url.Parse("https://charts.upbound.io/stable")
	uxpUnstableRepoURL, _ = url.Parse("https://charts.upbound.io/main")
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(ctx *kong.Context) error {
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}
	ctx.Bind(&install.Context{
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

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"UXP_NAMESPACE" default:"upbound-system" help:"Kubernetes namespace for UXP."`
}
