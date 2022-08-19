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

// Cmd contains commands for interacting with xpkgs.
type Cmd struct {
	Build     buildCmd     `cmd:"" group:"xpkg" help:"Build a package."`
	XPExtract XPExtractCmd `cmd:"" group:"xpkg" hidden:"" help:"Extract package contents into a Crossplane cache compatible format. Fetches from a remote registry by default."`
	Init      initCmd      `cmd:"" group:"xpkg" help:"Initialize a package."`
	Dep       depCmd       `cmd:"" group:"xpkg" help:"Manage package dependencies."`
	Push      pushCmd      `cmd:"" group:"xpkg" help:"Push a package."`
}
