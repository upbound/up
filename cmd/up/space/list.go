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
	"net/http"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	uerrors "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/accounts"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

var (
	spacelistFieldNames = []string{"NAME", "MODE", "PROVIDER", "REGION"}

	errListSpaces = "unable to list Upbound Spaces"
)

// listCmd lists all of the spaces in Upbound.
type listCmd struct {
	Upbound upbound.Flags `embed:""`

	kc client.Client
	ac *accounts.Client
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
	c.ac = accounts.NewClient(cfg)

	kc, err := client.New(ctrlCfg, client.Options{})
	if err != nil {
		return errors.Wrap(err, errListSpaces)
	}
	c.kc = kc

	kongCtx.Bind(upCtx)
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))

	return nil
}

// Run executes the list command.
func (c *listCmd) Run(ctx context.Context, printer upterm.Printer, p pterm.TextPrinter, upCtx *upbound.Context) error {
	a, err := upbound.GetOrganization(ctx, c.ac, upCtx.Account)
	var uerr *uerrors.Error
	if errors.As(err, &uerr) {
		if uerr.Status == http.StatusUnauthorized {
			p.Println("You must be logged in and authorized to list Upbound Cloud Spaces")
			return uerr
		}
	}

	if err != nil {
		return errors.Wrap(err, errListSpaces)
	}

	var l upboundv1alpha1.SpaceList
	err = c.kc.List(ctx, &l, &client.ListOptions{Namespace: a.Organization.Name})
	if err != nil {
		return errors.Wrap(err, errListSpaces)
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
