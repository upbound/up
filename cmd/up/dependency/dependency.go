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

package dependency

// Cmd contains commands for dependency cmd
type Cmd struct {
	Add addCmd `cmd:"" help:"Add a Package to current Configuration."`
}

func (c *Cmd) Help() string {
	return `
The dependency command manages crossplane package dependencies of the package
in the current directory. It caches package information in a local file system
cache (by default in ~/.up/cache), to be used e.g. for the upbound language
server.
`
}
