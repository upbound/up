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

	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upterm"
)

const (
	confirmStr      = "DESTROY CONFIRMED"
	nsUpboundSystem = "upbound-system"
)

// AfterApply sets default values in command after assignment and validation.
func (c *destroyCmd) AfterApply(insCtx *install.Context) error {
	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	if !c.Confirmed {
		prompter := input.NewPrompter()
		pterm.Println()
		pterm.FgRed.Println("******************** DESTRUCTIVE COMMAND ********************")
		pterm.FgRed.Println("****************** DATA-DESTRUCTION WARNING *****************")
		pterm.Println()
		pterm.Warning.Println("Destroying Spaces is a destructive command that will destroy data and oprhan resources.")
		pterm.Warning.Println("Before proceeding ensure that Managed Resources in Control Planes have been deleted.")
		pterm.Warning.Println("All Spaces components including Control Planes will be destroyed.")
		pterm.Println()
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

	kClient, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = kClient

	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		spacesChart,
		c.Registry,
		helm.WithNamespace(ns),
		helm.IsOCI())
	if err != nil {
		return err
	}
	c.mgr = mgr
	return nil
}

// destroyCmd uninstalls Upbound.
type destroyCmd struct {
	mgr     install.Manager
	kClient kubernetes.Interface

	commonParams

	Confirmed bool `name:"yes-really-delete-spaces-and-all-data" type:"bool" help:"Bypass safety checks and destroy Spaces"`
}

// Run executes the uninstall command.
func (c *destroyCmd) Run(insCtx *install.Context) error {
	if err := c.mgr.Uninstall(); err != nil {
		return err
	}
	return c.kClient.CoreV1().Namespaces().Delete(context.Background(), nsUpboundSystem, v1.DeleteOptions{})
}
