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
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

type ctpDeleter interface {
	Delete(ctx context.Context, ctp types.NamespacedName) error
}

// deleteCmd deletes a control plane on Upbound.
type deleteCmd struct {
	Name  string `arg:"" help:"Name of control plane." predictor:"ctps"`
	Group string `short:"g" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current profile."`

	client ctpDeleter
}

// AfterApply sets default values in command after assignment and validation.
func (c *deleteCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	return nil
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
	_, currentProfile, ctp, err := upCtx.Cfg.GetCurrentContext(ctx)
	if err != nil {
		return err
	}
	if currentProfile == nil {
		return errors.New(profile.NoSpacesContextMsg)
	}
	if ctp.Namespace == "" {
		return errors.New(profile.NoGroupMsg)
	}
	if ctp.Name != "" {
		return errors.New("Cannot delete control planes from inside a control plane, use `up ctx ..` to switch to a group level.")
	}

	// create client
	restConfig, _, err := currentProfile.GetSpaceRestConfig()
	if err != nil {
		return err
	}
	cl, err := ctrlclient.New(restConfig, ctrlclient.Options{})
	if err != nil {
		return err
	}

	ns := ctp.Namespace
	if c.Group != "" {
		ns = c.Group
	}

	ctpToDelete := &spacesv1beta1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: ns,
		},
	}
	if err := cl.Delete(ctx, ctpToDelete); err != nil {
		if kerrors.IsNotFound(err) {
			p.Printfln("Control plane %s not found", c.Name)
			return nil
		}
		return err
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
