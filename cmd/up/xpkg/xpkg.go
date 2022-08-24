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

package xpkg

import (
	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/feature"
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// Cmd contains commands for interacting with xpkgs.
type Cmd struct {
	Build     buildCmd     `cmd:"" help:"Build a package."`
	XPExtract xpExtractCmd `cmd:"" maturity:"alpha" help:"Extract package contents into a Crossplane cache compatible format. Fetches from a remote registry by default."`
	Init      initCmd      `cmd:"" help:"Initialize a package."`
	Dep       depCmd       `cmd:"" help:"Manage package dependencies."`
	Push      pushCmd      `cmd:"" help:"Push a package."`
}
