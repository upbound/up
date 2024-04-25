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

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

var (
	spacelistFieldNames = []string{"NAME", "MODE", "PROVIDER", "REGION"}
)

// listCmd lists all of the spaces in Upbound.
type listCmd struct {
	Upbound upbound.Flags `embed:""`
}

// AfterApply sets default values in command after assignment and validation.
func (c *listCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}

	ctrlCfg, err := upCtx.BuildControllerClientConfig()
	if err != nil {
		return err
	}

	kongCtx.Bind(upCtx)
	kongCtx.Bind(ctrlCfg)
	kongCtx.Bind(accounts.NewClient(cfg))

	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))

	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context, ac *accounts.Client, rest *rest.Config) error {
	a, err := upbound.GetAccount(ctx, ac, upCtx.Account)
	if err != nil {
		return err
	}

	sc, err := client.New(rest, client.Options{})
	if err != nil {
		return err
	}

	var l upboundv1alpha1.SpaceList
	err = sc.List(ctx, &l, &client.ListOptions{Namespace: a.Organization.Name})
	if err != nil {
		return err
	}

	if len(l.Items) == 0 {
		p.Println("No spaces found")
		return nil
	}

	return printer.Print(l.Items, spacelistFieldNames, extractSpaceListFields)
}

func extractSpaceListFields(obj any) []string {
	space, ok := obj.(upboundv1alpha1.Space)
	if !ok {
		return []string{"unknown", "unknown", "", ""}
	}

	provider, region := "", ""
	if space.Spec.Provider != nil {
		provider = string(*space.Spec.Provider)
	}

	if space.Spec.Region != nil {
		region = string(*space.Spec.Region)
	}

	mode := space.ObjectMeta.Labels[upboundv1alpha1.SpaceModeLabelKey]

	return []string{
		space.GetObjectMeta().GetName(),
		mode,
		provider,
		region,
	}
}
