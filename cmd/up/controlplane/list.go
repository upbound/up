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
	xpcommonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/pterm/pterm"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up-sdk-go/service/common"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	maxItems = 100
)

const (
	notAvailable = "n/a"
)

var cloudFieldNames = []string{"NAME", "ID", "STATUS", "DEPLOYED CONFIGURATION", "CONFIGURATION STATUS"}
var spacesFieldNames = []string{"NAME", "ID", "STATUS", "MESSAGE", "CONNECTION NAME", "CONNECTION NAMESPACE"}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// listCmd list control planes in an account on Upbound.
type listCmd struct{}

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, cc *cp.Client, kube *dynamic.DynamicClient, upCtx *upbound.Context) error {
	if upCtx.Profile.IsSpace() {
		return c.runSpaces(printer, p, kube)
	}
	return c.runCloud(printer, p, cc, upCtx)
}

func (c *listCmd) runCloud(printer upterm.ObjectPrinter, p pterm.TextPrinter, cc *cp.Client, upCtx *upbound.Context) error {
	// TODO(hasheddan): we currently just max out single page size, but we
	// may opt to support limiting page size and iterating through pages via
	// flags in the future.
	cpList, err := cc.List(context.Background(), upCtx.Account, common.WithSize(maxItems))
	if err != nil {
		return err
	}

	if len(cpList.ControlPlanes) == 0 {
		p.Printfln("No control planes found in %s", upCtx.Account)
		return nil
	}
	return printer.Print(cpList.ControlPlanes, cloudFieldNames, extractCloudFields)
}

func (c *listCmd) runSpaces(printer upterm.ObjectPrinter, p pterm.TextPrinter, kube *dynamic.DynamicClient) error {
	// NOTE: It would be convenient if we could import the ControlPlane types
	// and SchemeBuilder from upbound/mxe and use them to build a client that
	// returns structured data, but it's a private repo. Instead we use a dynamic
	// client and unstructured objects.
	cpList, err := getControlPlanes(context.Background(), kube)
	if err != nil {
		return err
	}
	if len(cpList.Items) == 0 {
		p.Println("No control planes found")
		return nil
	}
	return printer.Print(cpList.Items, spacesFieldNames, extractSpacesFields)
}

// Hey Taylor -- I was thinking of moving this function to a new package.
// That was the one thing I wanted to do before putting this up for a PR.
func getControlPlanes(ctx context.Context, kube *dynamic.DynamicClient) (*unstructured.UnstructuredList, error) {
	cpList, err := kube.Resource(resources.ControlPlaneGVR).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return cpList, nil
}

func extractCloudFields(obj any) []string {
	c := obj.(cp.ControlPlaneResponse)
	var cfgName string
	var cfgStatus string
	// All Upbound managed control planes in an account should be associated to a configuration.
	// However, we should still list all control planes and indicate where this isn't the case.
	if c.ControlPlane.Configuration.Name != nil && c.ControlPlane.Configuration != EmptyControlPlaneConfiguration() {
		cfgName = *c.ControlPlane.Configuration.Name
		cfgStatus = string(c.ControlPlane.Configuration.Status)
	} else {
		cfgName, cfgStatus = notAvailable, notAvailable
	}
	return []string{c.ControlPlane.Name, c.ControlPlane.ID.String(), string(c.Status), cfgName, cfgStatus}
}

func extractSpacesFields(obj any) []string {
	id, readyStatus := "unknown", "unknown"

	uc, ok := obj.(unstructured.Unstructured)
	if !ok {
		return []string{"", id, readyStatus}
	}

	ctp := resources.ControlPlane{Unstructured: uc}

	cnd := ctp.GetCondition(xpcommonv1.TypeReady)
	ref := ctp.GetConnectionSecretToReference()
	if ref == nil {
		ref = &xpcommonv1.SecretReference{}
	}

	return []string{ctp.GetName(), ctp.GetControlPlaneID(), string(cnd.Reason), cnd.Message, ref.Name, ref.Namespace}
}
