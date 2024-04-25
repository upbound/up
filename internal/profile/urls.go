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
	"regexp"

	"k8s.io/apimachinery/pkg/types"
)

var (
	// Matches https://00.000.000.0.nip.io/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp1/k8s
	newControlPlanePathRE = regexp.MustCompile(`^(?P<base>.+)/apis/spaces.upbound.io/(?P<version>v[^/]+)/namespaces/(?P<namespace>[^/]+)/controlplanes/(?P<controlplane>[^/]+)/k8s$`)
	// Matches https://spaces-foo.upboundrocks.cloud/v1/controlplanes/acmeco/default/ctp/k8s
	oldControlPlanePathRE = regexp.MustCompile(`^(?P<base>.+)/v1/control[pP]lanes/(?P<account>[^/]+)/(?P<namespace>[^/]+)/(?P<controlplane>[^/]+)/k8s$`)
)

// ParseSpacesK8sURL parses a URL and returns the namespace and controlplane name.
func ParseSpacesK8sURL(url string) (types.NamespacedName, bool) {
	m := newControlPlanePathRE.FindStringSubmatch(url)
	if m == nil {
		m = oldControlPlanePathRE.FindStringSubmatch(url)
		if m == nil {
			return types.NamespacedName{}, false
		}
		return types.NamespacedName{Namespace: m[3], Name: m[4]}, true
	}
	return types.NamespacedName{Namespace: m[3], Name: m[4]}, true
}
