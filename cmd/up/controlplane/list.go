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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up-sdk-go/service/configurations"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/controlplane/cloud"
	"github.com/upbound/up/internal/controlplane/space"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	notAvailable = "n/a"
)

var cloudFieldNames = []string{"NAME", "ID", "STATUS", "CONFIGURATION", "CONFIGURATION STATUS", "CONNECTION NAME", "CONNECTION NAMESPACE"}
var spacesFieldNames = []string{"NAME", "ID", "STATUS", "MESSAGE", "CONNECTION NAME", "CONNECTION NAMESPACE"}

type ctpLister interface {
	List(ctx context.Context) (*resources.ControlPlaneList, error)
}

// listCmd list control planes in an account on Upbound.
type listCmd struct {
	client ctpLister
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {

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

// Run executes the list command.
func (c *listCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context) error {
	l, err := c.client.List(context.Background())
	if err != nil {
		return err
	}

	if len(l.Items) == 0 {
		p.Println("No control planes found")
		return nil
	}
	printer.Print(l.Items, spacesFieldNames, extractSpacesFields)
	return nil
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
