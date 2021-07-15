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

package uxp

import "os"

// ChartParams common parameters for installing/upgrading charts
type ChartParams struct {
	Unstable bool              `help:"Allow installing unstable UXP versions."`
	Set      map[string]string `help:"Set Helm parameters."`
	File     *os.File          `short:"f" help:"Helm parameters file."`
	Chart    *os.File          `help:"Chart archive file."`
}
