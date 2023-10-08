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
)

type ctpDeleter interface {
	Delete(ctx context.Context, name string) error
}

// deleteCmd deletes a control plane on Upbound.
type deleteCmd struct {
	Name string `arg:"" help:"Name of control plane." predictor:"ctps"`

	client ctpDeleter
}

// AfterApply sets default values in command after assignment and validation.
func (c *deleteCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {

	if upCtx.Profile.IsSpace() {
		kubeconfig, err := upCtx.Profile.GetKubeClientConfig()
		if err != nil {
			return err
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
	return nil
}

// Run executes the delete command.
func (c *deleteCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if err := c.client.Delete(context.Background(), c.Name); err != nil {
		if controlplane.IsNotFound(err) {
			p.Printfln("Control plane %s not found", c.Name)
			return nil
		}
		return err
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
