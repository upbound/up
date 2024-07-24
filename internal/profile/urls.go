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
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/types"
)

var (
	// Matches https://00.000.000.0.nip.io/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp1/k8s
	controlPlaneRE = regexp.MustCompile(`^(?P<base>.+)/apis/spaces.upbound.io/(?P<version>v[^/]+)/namespaces/(?P<namespace>[^/]+)/controlplanes/(?P<controlplane>[^/]+)/k8s$`)
	// Matches https://spaces-foo.upboundrocks.cloud/v1/controlplanes/acmeco/default/ctp/k8s
	legacyControlPlanePathRE = regexp.MustCompile(`^(?P<base>.+)/v1/control[pP]lanes/(?P<account>[^/]+)/(?P<namespace>[^/]+)/(?P<controlplane>[^/]+)/k8s$`)
)

// ParseSpacesK8sURL parses a URL and returns the namespace and optionally the
// controlplane name (if specified).
func ParseSpacesK8sURL(url string) (base string, ctp types.NamespacedName, matches bool) {
	m := controlPlaneRE.FindStringSubmatch(url)
	if m == nil {
		return "", types.NamespacedName{}, false
	}

	baseInd := controlPlaneRE.SubexpIndex("base")
	nsInd := controlPlaneRE.SubexpIndex("namespace")
	nameInd := controlPlaneRE.SubexpIndex("controlplane")
	return m[baseInd], types.NamespacedName{Namespace: m[nsInd], Name: m[nameInd]}, true
}

// ParseMCPK8sURL attempts to parse a legacy MCP URL and returns the name of the
// matching control plane if it matches
func ParseMCPK8sURL(url string) (ctp types.NamespacedName, matches bool) {
	m := legacyControlPlanePathRE.FindStringSubmatch(url)
	if m == nil {
		return types.NamespacedName{}, false
	}

	nsInd := legacyControlPlanePathRE.SubexpIndex("namespace")
	nameInd := legacyControlPlanePathRE.SubexpIndex("controlplane")
	return types.NamespacedName{Namespace: m[nsInd], Name: m[nameInd]}, true
}

func ToSpacesK8sURL(ingress string, ctp types.NamespacedName) string {
	// pointed directly at the space
	if ctp.Name == "" {
		return fmt.Sprintf("https://%s", ingress)
	}

	// pointed at a control plane
	return fmt.Sprintf("https://%s/apis/spaces.upbound.io/v1beta1/namespaces/%s/controlplanes/%s/k8s", ingress, ctp.Namespace, ctp.Name)
}
