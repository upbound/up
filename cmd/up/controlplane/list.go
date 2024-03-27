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

package controlplane

import (
	"context"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up-sdk-go/service/configurations"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/controlplane/cloud"
	"github.com/upbound/up/internal/controlplane/space"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

type ctpLister interface {
	List(ctx context.Context, namespace string) ([]*controlplane.Response, error)
}

// listCmd list control planes in an account on Upbound.
type listCmd struct {
	Group     string `short:"g" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current profile."`
	AllGroups bool   `short:"A" default:"false" help:"List control planes across all groups."`

	client ctpLister
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	if upCtx.Profile.IsSpace() {
		kubeconfig, ns, err := upCtx.Profile.GetSpaceRestConfig()
		if err != nil {
			return err
		}
		if c.Group == "" {
			c.Group = ns
		}

		client, err := dynamic.NewForConfig(kubeconfig)
		if err != nil {
			return err
		}
		c.client = space.New(client)
	} else {
		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return err
		}
		ctpclient := cp.NewClient(cfg)
		cfgclient := configurations.NewClient(cfg)

		c.client = cloud.New(ctpclient, cfgclient, upCtx.Account)
	}

	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context) error {
	l, err := c.client.List(ctx, c.deriveGroup())
	if controlplane.IsNotFound(err) {
		p.Printfln("No Control planes found in %s group", c.deriveGroup())
		return nil
	}
	if err != nil {
		return err
	}

	if len(l) == 0 {
		p.Println("No control planes found")
		return nil
	}

	return tabularPrint(l, printer, upCtx)
}

func (c *listCmd) deriveGroup() string {
	if c.AllGroups {
		return ""
	}
	return c.Group
}
