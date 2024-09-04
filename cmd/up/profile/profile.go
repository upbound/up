// Copyright 2022 Upbound Inc
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
	"github.com/posener/complete"

	"github.com/upbound/up/cmd/up/profile/config"
	"github.com/upbound/up/internal/upbound"
)

// Cmd contains commands for Upbound Profiles.
type Cmd struct {
	Current currentCmd `cmd:"" help:"Get current Upbound Profile."`
	List    listCmd    `cmd:"" help:"List Upbound Profiles."`
	Use     useCmd     `cmd:"" help:"Select an Upbound Profile as the default."`
	View    viewCmd    `cmd:"" help:"View the Upbound Profile settings across profiles."`
	Config  config.Cmd `cmd:"" help:"Interact with the current Upbound Profile's config."`

	// Deprecated
	Set setCmd `cmd:"" help:"Deprecated: Set an Upbound Profile for use with a Space."`

	Flags upbound.Flags `embed:""`
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)
	return nil
}

func PredictProfiles() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}

		profiles, err := upCtx.Cfg.GetUpboundProfiles()
		if err != nil {
			return nil
		}

		data := make([]string, 0)

		for name := range profiles {
			data = append(data, name)
		}
		return data
	})
}
