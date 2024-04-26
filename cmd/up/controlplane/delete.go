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
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

// deleteCmd deletes a control plane on Upbound.
type deleteCmd struct {
	Name string `arg:"" help:"Name of control plane." predictor:"ctps"`
}

// AfterApply sets default values in command after assignment and validation.
func (c *deleteCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	return nil
}

// Run executes the delete command.
func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, client client.Client) error {
	ns, _, err := upCtx.Kubecfg.Namespace()
	if err != nil {
		return errors.Wrap(err, "error getting namespace")
	}

	ctp := &spacesv1beta1.ControlPlane{
		ObjectMeta: v1.ObjectMeta{
			Name:      c.Name,
			Namespace: ns,
		},
	}

	if err := client.Delete(ctx, ctp); err != nil {
		if kerrors.IsNotFound(err) {
			p.Printfln("Control plane %s not found", c.Name)
			return nil
		}
		return errors.Wrap(err, "error deleting control plane")
	}
	p.Printfln("%s deleted", c.Name)
	return nil
}
