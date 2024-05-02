// Copyright 2023 Upbound Inc
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
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/upbound"

	_ "embed"
)

type setCmd struct {
	Space spaceCmd `cmd:"" help:"Deprecated: Set an Upbound Profile for use with a Space."`
}

type spaceCmd struct {
	Kube upbound.KubeFlags `embed:""`
}

func (c *spaceCmd) Run(ctx context.Context) error {
	return errors.New("this command has been removed. To access a space, simply set KUBECONFIG to a valid space cluster")
}
