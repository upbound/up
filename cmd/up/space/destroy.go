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

package space

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	confirmStr      = "CONFIRMED"
	nsUpboundSystem = "upbound-system"
)

// destroyCmd uninstalls Upbound.
type destroyCmd struct {
	Upbound  upbound.Flags     `embed:""`
	Registry registryFlags     `embed:""`
	Kube     upbound.KubeFlags `embed:""`

	Confirmed bool `name:"yes-really-delete-space-and-all-data" type:"bool" help:"Bypass safety checks and destroy Spaces"`
	Orphan    bool `name:"orphan" type:"bool" help:"Remove Space components but retain Control Planes and data"`
}

// AfterApply sets default values in command after assignment and validation.
func (c *destroyCmd) AfterApply(kongCtx *kong.Context) error {
	if err := c.Kube.AfterApply(); err != nil {
		return err
	}

	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	c.confirm()

	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}

	kubeconfig, err := c.getKubeconfig(upCtx)
	if err != nil {
		return err
	}

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	kongCtx.Bind(kClient)

	with := []helm.InstallerModifierFn{
		helm.WithNamespace(ns),
		helm.IsOCI(),
	}
	if c.Orphan {
		with = append(with, helm.WithNoHooks())
	}

	mgr, err := helm.NewManager(kubeconfig,
		spacesChart,
		c.Registry.Repository,
		with...,
	)
	if err != nil {
		return err
	}
	kongCtx.Bind(mgr)

	return nil
}

// confirm prompts for confirmation and exits if the user declines.
func (c *destroyCmd) confirm() {
	if c.Confirmed {
		return
	}
	if c.Orphan {
		pterm.Info.Println()
		pterm.Info.Println("Removing Space API components.")
		pterm.Info.Println("Control Planes will continue to run and no data will be lost")
		pterm.Info.Println()
	} else {
		pterm.Println()
		pterm.FgRed.Println("******************** DESTRUCTIVE COMMAND ********************")
		pterm.FgRed.Println("********************* DATA-LOSS WARNING *********************")
		pterm.Println()
		pterm.Warning.Println("Destroying Spaces is a destructive command that will destroy data and orphan resources.")
		pterm.Warning.Println("Before proceeding ensure that Managed Resources in Control Planes have been deleted.")
		pterm.Warning.Println("All Spaces components including Control Planes will be destroyed.")
		pterm.Println()
		pterm.Warning.Println("If you want to retain data, abort and run 'up space destroy --orphan'")
		pterm.Println()
	}

	prompter := input.NewPrompter()
	in, err := prompter.Prompt(fmt.Sprintf("To proceed, type: %q", confirmStr), false)
	if err != nil {
		pterm.Error.Printfln("error getting user confirmation: %v", err)
		os.Exit(1)
	}
	if in != confirmStr {
		pterm.Error.Println("Destruction was not confirmed")
		os.Exit(10)
	}
}

// getKubeconfig returns the kubeconfig from flags if provided, otherwise the
// kubeconfig from the active profile.
func (c *destroyCmd) getKubeconfig(upCtx *upbound.Context) (*rest.Config, error) {
	if c.Kube.Kubeconfig != "" || c.Kube.Context != "" {
		return c.Kube.GetConfig(), nil
	}
	if !upCtx.Profile.IsSpacesProfile() {
		return nil, fmt.Errorf("destroy is not supported for non-Spaces profile %q", upCtx.ProfileName)
	}
	return upCtx.Profile.GetKubeClientConfig()
}

// Run executes the uninstall command.
func (c *destroyCmd) Run(kClient *kubernetes.Clientset, mgr *helm.Installer) error {
	if err := mgr.Uninstall(); err != nil {
		return err
	}

	// leave `upbound-system` namespace in place since there are secrets, configmaps, etc,
	// used by controlplanes
	if c.Orphan {
		return nil
	}

	return kClient.CoreV1().Namespaces().Delete(context.Background(), nsUpboundSystem, v1.DeleteOptions{})
}
