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
	"errors"
	"strings"

	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CloudType string

type CloudConfig struct {
	SpacesValues  map[string]string
	PublicIngress bool
}

const (
	AmazonEKS CloudType = "eks"
	AzureAKS  CloudType = "aks"
	Generic   CloudType = "generic"
	GoogleGKE CloudType = "gke"
	Kind      CloudType = "kind"

	ClusterTypeStr = "clusterType"
)

var vendorDefaults = map[CloudType]map[string]string{
	AmazonEKS: {
		ClusterTypeStr: string(AmazonEKS),
	},
	AzureAKS: {
		ClusterTypeStr: string(AzureAKS),
	},
	GoogleGKE: {
		ClusterTypeStr: string(GoogleGKE),
	},
	Kind: {
		ClusterTypeStr: string(Kind),
	},

	Generic: {},
}

func (ct *CloudType) getSpaceValues() map[string]string {
	if v, ok := vendorDefaults[*ct]; ok {
		return v
	}
	return nil
}

func (ct *CloudType) Defaults() CloudConfig {
	publicIngress := true
	if *ct == Generic || *ct == Kind {
		publicIngress = false
	}
	return CloudConfig{
		SpacesValues:  ct.getSpaceValues(),
		PublicIngress: publicIngress,
	}
}

func GetConfig(kClient kubernetes.Interface, override string) (*CloudConfig, error) {
	if kClient == nil {
		return nil, errors.New("no kubernetes client")
	}
	var cloud CloudType
	if override != "" {
		cloud = CloudType(strings.ToLower(override))
	} else {
		cloud = detectKubernetes(kClient)
	}
	if cloud == Generic || cloud == Kind {
		pterm.Info.Printfln("Setting defaults for vanilla Kubernetes (type %s)", string(cloud))
	} else {
		pterm.Info.Printfln("Applying settings for Managed Kubernetes on %s", strings.ToUpper(string(cloud)))
	}
	return &CloudConfig{
		SpacesValues:  cloud.Defaults().SpacesValues,
		PublicIngress: cloud.Defaults().PublicIngress,
	}, nil
}

// detectKubernetes looks at a nodes provider to determine what type of cluster
// is running. Since Spaces doesn't directly use Node objects, requiring Nodes
// to use the installer would be incorrect. This is a "best effort" attempt to
// add some CLI sugar, so reacting to an error seems suboptimal, especially if the
// installer doesn't have RBAC permissions to list nodes.
func detectKubernetes(kClient kubernetes.Interface) CloudType {
	// EKS and Kind are _harder_ to detect based on version, so look at node labels.
	ctx := context.Background()
	if nodes, err := kClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		for _, n := range nodes.Items {
			providerPrefix := strings.Split(n.Spec.ProviderID, "://")[0]
			switch providerPrefix {
			case "azure":
				return AzureAKS
			case "aws":
				return AmazonEKS
			case "gce":
				return GoogleGKE
			case "kind":
				return Kind
			}
		}
	}

	return Generic
}
