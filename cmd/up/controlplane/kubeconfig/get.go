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
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *getCmd) AfterApply(experimental bool) error {
	c.stdin = os.Stdin

	if !experimental {
		u, err := uuid.Parse(c.ID)
		if err != nil {
			return err
		}
		c.id = u
	}
	return nil
}

// getCmd gets kubeconfig data for an Upbound control plane.
type getCmd struct {
	stdin io.Reader

	File  string   `type:"path" short:"f" help:"File to merge kubeconfig."`
	Proxy *url.URL `env:"UP_PROXY" default:"https://proxy.upbound.io/controlPlanes" help:"Endpoint used for Upbound Proxy."`
	Token string   `required:"" help:"API token used to authenticate."`

	id uuid.UUID

	ID string `arg:"" name:"control-plane-ID" required:"" help:"ID of control plane. ID is name if using experimental MCP API."`
}

// Run executes the get command.
func (c *getCmd) Run(experimental bool, upCtx *upbound.Context) error {
	// TODO(hasheddan): consider implementing a custom decoder
	if c.Token == "-" {
		b, err := io.ReadAll(c.stdin)
		if err != nil {
			return err
		}
		c.Token = strings.TrimSpace(string(b))
	}

	if experimental {
		c.Proxy.Path = fmt.Sprintf("/v1/%s", c.Proxy.Path)
		return kube.BuildControlPlaneKubeconfig(c.Proxy, c.ID, c.Token, c.File)
	}

	return kube.BuildControlPlaneKubeconfig(c.Proxy, c.id.String(), c.Token, c.File)
}
