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

package sos

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/migration"
)

// AfterApply constructs and binds Upbound specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	cfg, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	kongCtx.Bind(&migration.Context{
		Kubeconfig: cfg,
	})
	return nil
}

type Cmd struct {
	Export exportCmd `cmd:"" help:"Export an sos report of a Crossplane or Universal Crossplane (XP/UXP) control plane into an archive."`

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
}

func (c *Cmd) Help() string {
	return `
The 'sos' command collects configuration details, system information and diagnostic information from a Crossplane or Universal Crossplane (XP/UXP) control plane.
For instance: the running providers, functions, and system and service configuration files.
The command stores this output in the resulting archive.

For detailed information on each command and its options, use the '--help' flag with the specific command (e.g., 'up alpha sos export --help').
`
}
