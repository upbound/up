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

package group

import (
	"context"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

// createCmd creates a group in a space.
type createCmd struct {
	Name string `arg:"" required:"" help:"Name of group."`
}

// Run executes the create command.
func (c *createCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, upCtx *upbound.Context, client client.Client, p pterm.TextPrinter) error { // nolint:gocyclo
	// create group
	group := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Name,
			Labels: map[string]string{
				spacesv1beta1.ControlPlaneGroupLabelKey: "true",
			},
		},
	}

	if err := client.Create(ctx, &group); err != nil {
		return err
	}

	p.Printfln("%s created", c.Name)
	return nil
}
