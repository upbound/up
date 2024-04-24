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
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

const (
	kubeconfigFmt = "kubeconfig-%s"
)

// createCmd creates a control plane on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of control plane."`

	ConfigurationName *string `help:"The optional name of the Configuration."`
	Description       string  `short:"d" help:"Description for control plane."`

	SecretName string `help:"The name of the control plane's secret. Defaults to 'kubeconfig-{control plane name}'. Only applicable for Space control planes."`
	Group      string `short:"g" help:"The control plane group that the control plane is contained in. This defaults to the group specified in the current profile."`
}

// AfterApply sets default values in command after assignment and validation.
func (c *createCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
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
		return errors.New("Cannot create control planes from inside a control plane, use `up ctx ..` to switch to a group level.")
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

	if c.SecretName == "" {
		c.SecretName = fmt.Sprintf(kubeconfigFmt, c.Name)
	}

	// TODO(erhan): check if we need to handle c.Description and c.ConfigurationName parameters
	ctpToCreate := &spacesv1beta1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: ns,
		},
		Spec: spacesv1beta1.ControlPlaneSpec{
			WriteConnectionSecretToReference: &spacesv1beta1.SecretReference{
				Name:      c.SecretName,
				Namespace: ns,
			},
			// TODO(erhan): check if we need to specify PublishConnectionDetailsTo
		},
	}

	if errCreate := cl.Create(ctx, ctpToCreate); errCreate != nil {
		return errCreate
	}

	p.Printfln("%s created", c.Name)
	return nil
}
