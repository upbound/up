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

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/upbound"
)

func DeriveState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config) (NavigationState, error) {
	kubeconfig, err := upCtx.Kubecfg.RawConfig()
	if err != nil {
		return nil, err
	}

	ctp, exists := upCtx.ParseCurrentSpaceContextURL()

	// not pointed at any context
	if !exists {
		return &Organizations{}, nil
	}

	// derive navigation state
	switch {
	case ctp.Namespace != "" && ctp.Name != "":
		return &ControlPlane{
			group: Group{
				space: Space{
					name: kubeconfig.CurrentContext,
				},
				name: ctp.Namespace,
			},
			name: ctp.Name,
		}, nil
	case ctp.Namespace != "":
		return &Group{
			space: Space{
				name: kubeconfig.CurrentContext,
			},
			name: ctp.Namespace,
		}, nil
	default:
		return &Space{
			name: kubeconfig.CurrentContext,
		}, nil
	}
}
