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

package profile

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetIngressHost returns the ingress host of the Spaces cfg points to. If the
// ingress is not configured, it returns an empty string.
func GetIngressHost(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
	mxpConfig := &corev1.ConfigMap{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "ingress-public", Namespace: "upbound-system"}, mxpConfig); err != nil {
		return "", nil, err
	}
	host = mxpConfig.Data["ingress-host"]
	ca = []byte(mxpConfig.Data["ingress-ca"])
	return host, ca, nil
}
