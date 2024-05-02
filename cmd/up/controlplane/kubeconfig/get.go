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

package kubeconfig

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// getCmd gets kubeconfig data for an Upbound control plane.
type getCmd struct {
	ConnectionSecretCmd

	File    string `type:"path" short:"f" help:"File to merge control plane kubeconfig into or to create. By default it is merged into the user's default kubeconfig. Use '-' to print it to stdout.'"`
	Context string `short:"c" help:"Context to use in the kubeconfig."`
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context) error {
	return errors.New("this command has been removed in favor of 'up ctx'")
}
