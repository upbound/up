// Copyright 2024 Upbound Inc
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

package space

import (
	"context"
	"fmt"
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/cmd/up/space/prerequisites"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	agentChart = "agent"

	// TODO(tnthornton) these can probably be replaced by our public chart
	// museum. This would allow us to use wildcards like mxp-connector.
	supportedVersion = "0.0.0-100.g216e157"
	agentRegistry    = "us-west1-docker.pkg.dev/orchestration-build/connect"

	// TODO(tnthornton) maybe move this to the agent chart?
	devConnectURL = "nats://connect.u5d.dev"
)

type attachCmd struct {
	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`

	helmMgr install.Manager
	prereqs *prerequisites.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface
	dClient dynamic.Interface
	quiet   config.QuietFlag

	ng names.NameGenerator

	Space string `arg:"" optional:"" help:"Name of the Upbound Space. If name is not a supplied, one is generated."`
}

func (c *attachCmd) AfterApply(kongCtx *kong.Context) error {
	registryURL, err := url.Parse(agentRegistry)
	if err != nil {
		return err
	}

	if err := c.Kube.AfterApply(); err != nil {
		return err
	}

	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	kubeconfig := c.Kube.GetConfig()

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = kClient

	dClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.dClient = dClient
	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		registryURL,
		helm.WithNamespace("upbound-connect"),
		helm.CreateNamespace(true),
		helm.IsOCI(),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.helmMgr = mgr

	c.ng = names.SimpleNameGenerator

	return nil
}

// Run executes the install command.
func (c *attachCmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	if c.Space == "" {
		c.Space = c.ng.GenerateName("space-")
	}
	fmt.Printf("Using Space name: %s\n", c.Space)

	attachSpinner, _ := upterm.CheckmarkSuccessSpinner.Start("Installing agent to connect to Upbound Console...")
	if err := c.helmMgr.Install(supportedVersion, map[string]any{
		"nats": map[string]any{
			"url": devConnectURL,
		},
		"space": c.Space,
		//TODO(tnthornton) reference account from profile.
	}); err != nil {
		return err
	}

	attachSpinner.Success()
	return nil
}
