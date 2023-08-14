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
	"strings"

	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
)

type CloudType string

const (
	AmazonEKS CloudType = "eks"
	AzureAKS  CloudType = "aks"
	Generic   CloudType = "generic"
	GoogleGKE CloudType = "gke"
)

func SetDefaults(s map[string]string, kClient kubernetes.Interface) (map[string]string, error) {
	cloud, err := detectKubernetes(kClient)
	if err != nil {
		return s, err
	}
	if cloud == Generic {
		pterm.Info.Println("Setting defaults for generic Kubernetes")
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
	ver, err := kClient.Discovery().ServerVersion()
	if err != nil {
		return Generic, err
	}

	// GKE has -gke in the git commit
	// Example:
	if strings.Contains(ver.GitVersion, "-gke.") {
		return GoogleGKE, nil
	}
	// EKS has -eks in the git commit
	if strings.Contains(ver.GitVersion, "-eks-") {
		return AmazonEKS, nil
	}
	// AKS has -aks in the git commit
	if strings.Contains(ver.GitVersion, "-aks") {
		return AzureAKS, nil
	}
	return Generic, nil
}
