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

package mirror

var artifacts = config{
	oci: []repository{
		{
			// spaces chart
			// version given by --version in cmd
			chart: "xpkg.upbound.io/spaces-artifacts/spaces",
			// searchPath list all supported uxp versions in spaces chart
			subCharts: []subChart{
				{
					pathNavigator: &uxpVersionsPath{},
					chart:         "xpkg.upbound.io/upbound/universal-crossplane",
					image:         "xpkg.upbound.io/upbound/crossplane",
				},
			},
			// all images with the same tag then spaces helm-chart
			// version given by --version in cmd
			images: []string{
				"xpkg.upbound.io/spaces-artifacts/hyperspace",
				"xpkg.upbound.io/spaces-artifacts/mxe-composition-templates",
				"xpkg.upbound.io/spaces-artifacts/mxp-authz-webhook",
				"xpkg.upbound.io/spaces-artifacts/mxp-benchmark",
				"xpkg.upbound.io/spaces-artifacts/mxp-charts",
				"xpkg.upbound.io/spaces-artifacts/mxp-control-plane",
				"xpkg.upbound.io/spaces-artifacts/mxp-host-cluster-worker",
				"xpkg.upbound.io/spaces-artifacts/mxp-host-cluster",
				"xpkg.upbound.io/spaces-artifacts/opentelemetry-collector-spaces",
				"xpkg.upbound.io/spaces-artifacts/provider-host-cluster",
			},
		},
	},
	// additional images for prerequisits
	images: []string{
		"xpkg.upbound.io/spaces-artifacts/mcp-connector:0.6.0",
		"xpkg.upbound.io/spaces-artifacts/mcp-connector-server:v0.6.0",
		"xpkg.upbound.io/crossplane-contrib/function-auto-ready:v0.2.1",
		"xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0",
		"xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v0.14.0",
	},
}
