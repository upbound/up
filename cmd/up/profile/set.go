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

package profile

import (
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upbound"
)

const (
	errSetProfile = "unable to set profile"
)

type setCmd struct {
	Space spaceCmd `cmd:"" help:"Create or update a profile for use with a Space."`
}

type spaceCmd struct {
	Kube upbound.KubeFlags `embed:""`
}

func (c *spaceCmd) AfterApply(kongCtx *kong.Context) error {
	return c.Kube.AfterApply()
}

func (c *spaceCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, config.Profile{
		Type:        config.SpacesProfileType,
		Kubeconfig:  c.Kube.Kubeconfig,
		KubeContext: c.Kube.GetContext(),
		// Carry over existing config.
		BaseConfig: upCtx.Profile.BaseConfig,
	}); err != nil {
		return err
	}
	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errSetProfile)
}
