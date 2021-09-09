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
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
)

const enterpriseChart = "enterprise"

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

// Cmd contains commands for managing enterprise.
type Cmd struct {
	Install   installCmd   `cmd:"" group:"enterprise" help:"Install enterprise."`
	Uninstall uninstallCmd `cmd:"" group:"enterprise" help:"Uninstall enterprise."`
	Upgrade   upgradeCmd   `cmd:"" group:"enterprise" help:"Upgrade enterprise."`

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"ENTERPRISE_NAMESPACE" default:"upbound-enterprise" help:"Kubernetes namespace for enterprise."`
}
