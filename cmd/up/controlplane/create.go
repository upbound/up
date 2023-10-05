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

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up-sdk-go/service/configurations"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"

	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
)

// createCmd creates a control plane on Upbound.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of control plane."`

	ConfigurationName string `help:"The name of the Configuration. This property is required for cloud control planes."`
	Description       string `short:"d" help:"Description for control plane."`

	SecretName      string `default:"kubeconfig-ctp" help:"The name of the control plane's secret. Only applicable for Space control planes."`
	SecretNamespace string `default:"default" help:"The name of namespace for the control plane's secret. Only applicable for Space control planes."`
}

// Run executes the create command.
func (c *createCmd) Run(p pterm.TextPrinter, cc *cp.Client, cfc *configurations.Client, kube *dynamic.DynamicClient, upCtx *upbound.Context) error {
	if upCtx.Profile.IsSpace() {
		return c.runSpaces(kube)
	}

	// Get the UUID from the Configuration name, if it exists.
	cfg, err := cfc.Get(context.Background(), upCtx.Account, c.ConfigurationName)
	if err != nil {
		return err
	}

	if _, err := cc.Create(context.Background(), upCtx.Account, &cp.ControlPlaneCreateParameters{
		Name:            c.Name,
		Description:     c.Description,
		ConfigurationID: cfg.ID,
	}); err != nil {
		return err
	}

	p.Printfln("%s created", c.Name)
	return nil
}

func (c *createCmd) runSpaces(kube *dynamic.DynamicClient) error {
	ctp := &resources.ControlPlane{}
	ctp.SetName(c.Name)
	ctp.SetGroupVersionKind(resources.ControlPlaneGVK)
	ctp.SetWriteConnectionSecretToReference(&v1.SecretReference{
		Name:      c.SecretName,
		Namespace: c.SecretNamespace,
	})

	_, err := kube.
		Resource(resources.ControlPlaneGVK.GroupVersion().WithResource("controlplanes")).
		Create(
			context.Background(),
			ctp.GetUnstructured(),
			metav1.CreateOptions{},
		)
	return err
}
