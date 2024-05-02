// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctx

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	upbound "github.com/upbound/up/internal/upbound"
)

const (
	upboundPreviousContextSuffix = "-previous"
)

// Accept upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the space.
func (s *Space) Accept(ctx context.Context, upCtx *upbound.Context, writer kubeContextWriter) (msg string, err error) {
	config, err := buildSpacesClient(s.ingress, s.ca, s.authInfo, types.NamespacedName{}).RawConfig()
	if err != nil {
		return "", err
	}
	if err := writer.Write(upCtx, &config); err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, config.CurrentContext, s.Breadcrumbs()), nil
}

// Accept upserts the "upbound" kubeconfig context and cluster to the chosen
// kubeconfig, pointing to the group.
func (g *Group) Accept(ctx context.Context, upCtx *upbound.Context, writer kubeContextWriter) (msg string, err error) {
	space := g.space

	config, err := buildSpacesClient(space.ingress, space.ca, space.authInfo, types.NamespacedName{Namespace: g.name}).RawConfig()
	if err != nil {
		return "", err
	}
	if err := writer.Write(upCtx, &config); err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, config.CurrentContext, g.Breadcrumbs()), nil
}

// Accept upserts a controlplane context and cluster to the chosen kubeconfig.
func (ctp *ControlPlane) Accept(ctx context.Context, upCtx *upbound.Context, writer kubeContextWriter) (msg string, err error) {
	space := ctp.group.space

	config, err := buildSpacesClient(space.ingress, space.ca, space.authInfo, ctp.NamespacedName()).RawConfig()
	if err != nil {
		return "", err
	}
	if err := writer.Write(upCtx, &config); err != nil {
		return "", err
	}

	return fmt.Sprintf(contextSwitchedFmt, config.CurrentContext, ctp.Breadcrumbs()), nil
}
