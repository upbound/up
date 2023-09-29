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
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

const (
	errSetProfile   = "unable to set profile"
	errUpdateConfig = "unable to update config file"
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
	setDefault := false

	// If profile name was not provided and no default exists, set name to
	// the default, and set this profile as the default profile.
	if upCtx.ProfileName == "" {
		upCtx.ProfileName = profile.DefaultName
		setDefault = true
	}

	prof := profile.Profile{
		Type:        profile.Space,
		Kubeconfig:  c.Kube.Kubeconfig,
		KubeContext: c.Kube.GetContext(),
		// Carry over existing config.
		BaseConfig: upCtx.Profile.BaseConfig,
	}

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, prof); err != nil {
		return errors.Wrap(err, errSetProfile)
	}

	if setDefault {
		if err := upCtx.Cfg.SetDefaultUpboundProfile(upCtx.ProfileName); err != nil {
			return errors.Wrap(err, errSetProfile)
		}
	}

	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, errUpdateConfig)
	}

	kubeconfigLocation := "default kubeconfig"
	if prof.Kubeconfig != "" {
		kubeconfigLocation = fmt.Sprintf("kubeconfig at %q", prof.Kubeconfig)
	}
	p.Printf("Profile %q updated to use context %q from the %s", upCtx.ProfileName, prof.KubeContext, kubeconfigLocation)
	if setDefault {
		p.Print(" and selected as the default profile")
	}
	p.Println()

	return nil
}
