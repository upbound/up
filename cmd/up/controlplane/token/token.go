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

package token

// Cmd contains commands for interacting with control plane tokens.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a control plane token."`
	Delete deleteCmd `cmd:"" help:"Delete a control plane token."`
	List   listCmd   `cmd:"" help:"List tokens for the control plane."`
}
