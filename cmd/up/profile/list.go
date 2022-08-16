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
	"sort"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(p pterm.TextPrinter, pt *pterm.TablePrinter, ctx *kong.Context, upCtx *upbound.Context) error {
	profiles, err := upCtx.Cfg.GetUpboundProfiles()
	if err != nil {
		return err
	}

	redacted := make(map[string]config.RedactedProfile)
	for k, v := range profiles {
		redacted[k] = config.RedactedProfile{Profile: v}
	}
	if len(redacted) == 0 {
		p.Println("No profiles found")
		return nil
	}

	// sort the redacted profiles by name so that we have a consistent listing
	profileNames := make([]string, 0, len(redacted))
	for k := range redacted {
		profileNames = append(profileNames, k)
	}
	sort.Strings(profileNames)

	dprofile, _, err := upCtx.Cfg.GetDefaultUpboundProfile()
	if err != nil {
		return err
	}

	data := make([][]string, len(redacted)+1)
	cursor := ""

	data[0] = []string{"CURRENT", "NAME", "TYPE", "ACCOUNT"}
	for i, name := range profileNames {
		if name == dprofile {
			cursor = "*"
		}
		prof := redacted[name]
		data[i+1] = []string{cursor, name, string(prof.Type), prof.Account}

		cursor = "" // reset cursor
	}

	return pt.WithHasHeader().WithData(data).Render()
}
