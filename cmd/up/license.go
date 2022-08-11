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

package main

import (
	"github.com/pterm/pterm"
)

// licenseCmd prints license information for using Up.
type licenseCmd struct{}

// Run executes the license command.
func (c *licenseCmd) Run(p pterm.TextPrinter) error {
	p.Println("By using Up, you are accepting to comply with terms and conditions in https://licenses.upbound.io/upbound-software-license.html")
	return nil
}
