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
	"encoding/json"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/upbound"
)

var (
	errNoProfiles = "No profiles found"
)

type viewCmd struct{}

// Run executes the list command.
func (c *viewCmd) Run(p pterm.TextPrinter, ctx *kong.Context, upCtx *upbound.Context) error {
	profiles, err := upCtx.Cfg.GetUpboundProfiles()
	if err != nil {
		p.Println(errNoProfiles)
		return nil // nolint:nilerr
	}

	redacted := make(map[string]config.RedactedProfile)
	for k, v := range profiles {
		redacted[k] = config.RedactedProfile{Profile: v}
	}

	b, err := json.MarshalIndent(redacted, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintln(ctx.Stdout, string(b))
	return nil
}
