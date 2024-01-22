// Copyright 2024 Upbound Inc
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

package migration

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/migration"
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	cfg, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	kongCtx.Bind(&migration.Context{
		Kubeconfig: cfg,
		Namespace:  c.Namespace,
	})
	return nil
}

type Cmd struct {
	Export exportCmd `cmd:"" help:"Export a control plane."`
	Import importCmd `cmd:"" help:"Import a control plane into a managed control plane."`

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"UXP_NAMESPACE" default:"upbound-system" help:"Kubernetes namespace for UXP."`
}
