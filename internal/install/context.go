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

package install

import (
	"os"

	"k8s.io/client-go/rest"
)

// Context includes common data that installer consumers may utilize.
type Context struct {
	Kubeconfig *rest.Config
	Namespace  string
}

// CommonParams are common parameters for installing and upgrading.
type CommonParams struct {
	Set    map[string]string `help:"Set parameters."`
	File   *os.File          `short:"f" help:"Parameters file."`
	Bundle *os.File          `help:"Local bundle path."`

	TokenFile *os.File `name:"token-file" required:"" help:"File containing authentication token."`
}
