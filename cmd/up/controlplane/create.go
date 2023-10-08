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

type ctpCreator interface {
	Create(ctx context.Context, name string, opts controlplane.Options) (*controlplane.Response, error)
}

// createCmd creates a control plane on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of control plane."`

	ConfigurationName string `help:"The name of the Configuration. This property is required for cloud control planes."`
	Description       string `short:"d" help:"Description for control plane."`

	SecretName      string `help:"The name of the control plane's secret. Defaults to 'kubeconfig-{control plane name}'. Only applicable for Space control planes."`
	SecretNamespace string `default:"default" help:"The name of namespace for the control plane's secret. Only applicable for Space control planes."`

	client ctpCreator
}

// AfterApply sets default values in command after assignment and validation.
func (c *createCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {

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

	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// Run executes the create command.
func (c *createCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	_, err := c.client.Create(
		context.Background(),
		c.Name,
		controlplane.Options{
			SecretName:      c.SecretName,
			SecretNamespace: c.SecretNamespace,
		},
	)
	if err != nil {
		return err
	}

	p.Printfln("%s created", c.Name)
	return nil
}
