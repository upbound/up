// Copyright 2023 Upbound Inc
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

package defaults

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CloudType string

const (
	AmazonEKS CloudType = "eks"
	AzureAKS  CloudType = "aks"
	Generic   CloudType = "generic"
	GoogleGKE CloudType = "gke"
	Kind      CloudType = "kind"
)

func SetDefaults(s map[string]string, kClient kubernetes.Interface) (map[string]string, error) {
	cloud, err := detectKubernetes(kClient)
	if err != nil {
		return s, err
	}
	if cloud == Generic || cloud == Kind {
		pterm.Info.Printfln("Setting defaults for vanilla Kubernetes (type %s)", string(cloud))
		return s, nil
	}

	pterm.Info.Printfln("Applying settings for Managed Kubernetes on %s", strings.ToUpper(string(cloud)))
	if s == nil {
		s = make(map[string]string)
	}

	// Set defaults
	d := map[string]string{
		"clusterType": string(cloud),
	}

	for k, v := range d {
		add := true
		for cs := range s {
			if cs == k {
				add = false
				break
			}
		}
		if add {
			s[k] = v
		}
	}
	return s, nil
}

func detectKubernetes(kClient kubernetes.Interface) (CloudType, error) {
	// EKS and Kind are _harder_ to detect based on version, so look at node labels.
	ctx := context.Background()
	if nodes, err := kClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		for _, n := range nodes.Items {
			providerPrefix := strings.Split(n.Spec.ProviderID, "://")[0]
			fmt.Println(providerPrefix)
			switch providerPrefix {
			case "azure":
				return AzureAKS, nil
			case "aws":
				return AmazonEKS, nil
			case "gce":
				return GoogleGKE, nil
			case "kind":
				return Kind, nil
			}
		}
	}

	return Generic, nil
}
