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

package prerequisites

import (
	"testing"

	"k8s.io/client-go/rest"

	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/stretchr/testify/require"

	"github.com/upbound/up/cmd/up/space/defaults"
	spacefeature "github.com/upbound/up/cmd/up/space/features"
	"github.com/upbound/up/cmd/up/space/prerequisites/certmanager"
	"github.com/upbound/up/cmd/up/space/prerequisites/ingressnginx"
	"github.com/upbound/up/cmd/up/space/prerequisites/opentelemetrycollector"
	"github.com/upbound/up/cmd/up/space/prerequisites/providers/helm"
	"github.com/upbound/up/cmd/up/space/prerequisites/providers/kubernetes"
	"github.com/upbound/up/cmd/up/space/prerequisites/uxp"
)

// Mock implementations or test helpers would be needed for uxp, kubernetes, helm, certmanager, ingressnginx, and opentelemetrycollector.
func TestNew(t *testing.T) {
	type args struct {
		config        *rest.Config
		defs          *defaults.CloudConfig
		setupFeatures func() *feature.Flags
		versionStr    string
	}

	type want struct {
		expectError     bool
		expectedErrMsg  string
		expectedPrereqs []string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"InvalidVersionFormat": {
			reason: "Testing invalid version format should return an error.",
			args: args{
				config:        &rest.Config{},
				defs:          &defaults.CloudConfig{},
				setupFeatures: func() *feature.Flags { return &feature.Flags{} },
				versionStr:    "invalid-version",
			},
			want: want{
				expectError:    true,
				expectedErrMsg: "invalid version format",
			},
		},
		"VersionLessThan170WithPrerequisites": {
			reason: "Testing version less than 1.7.0 should have specific prerequisites.",
			args: args{
				config: &rest.Config{},
				defs:   &defaults.CloudConfig{PublicIngress: false},
				setupFeatures: func() *feature.Flags {
					return &feature.Flags{}
				},
				versionStr: "v1.6.0",
			},
			want: want{
				expectError:     false,
				expectedPrereqs: []string{"uxp", "kubernetes", "helm", "certmanager", "ingressnginx"},
			},
		},
		"VersionGreaterThanOrEqualTo170WithoutCertainPrerequisites": {
			reason: "Testing version >= 1.7.0 should have specific prerequisites.",
			args: args{
				config: &rest.Config{},
				defs:   &defaults.CloudConfig{PublicIngress: true},
				setupFeatures: func() *feature.Flags {
					return &feature.Flags{}
				},
				versionStr: "v1.8.0",
			},
			want: want{
				expectError:     false,
				expectedPrereqs: []string{"certmanager", "ingressnginx"},
			},
		},
		"VersionIsRCof170WithoutCertainPrerequisites": {
			reason: "Testing a release candidate version of 1.7.0 should have specific prerequisites.",
			args: args{
				config: &rest.Config{},
				defs:   &defaults.CloudConfig{PublicIngress: true},
				setupFeatures: func() *feature.Flags {
					return &feature.Flags{}
				},
				versionStr: "v1.7.0-rc.0.221.gd1b9198d",
			},
			want: want{
				expectError:     false,
				expectedPrereqs: []string{"certmanager", "ingressnginx"},
			},
		},
		"FeatureEnabledWithAlphaSharedTelemetry": {
			reason: "Testing feature flag enabling alpha shared telemetry should add the opentelemetrycollector prerequisite.",
			args: args{
				config: &rest.Config{},
				defs:   &defaults.CloudConfig{},
				setupFeatures: func() *feature.Flags {
					flags := &feature.Flags{}
					flags.Enable(spacefeature.EnableAlphaSharedTelemetry)
					return flags
				},
				versionStr: "v1.8.0",
			},
			want: want{
				expectError:     false,
				expectedPrereqs: []string{"certmanager", "ingressnginx", "opentelemetrycollector"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			features := tc.args.setupFeatures() // Initialize feature flags using setup function
			manager, err := New(tc.args.config, tc.args.defs, features, tc.args.versionStr)

			if tc.want.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.want.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, manager)

				// Check if the prerequisites in the Manager match the expected types
				var prereqTypes []string
				for _, prereq := range manager.prereqs {
					switch prereq.(type) {
					case *uxp.UXP:
						prereqTypes = append(prereqTypes, "uxp")
					case *kubernetes.Kubernetes:
						prereqTypes = append(prereqTypes, "kubernetes")
					case *helm.Helm:
						prereqTypes = append(prereqTypes, "helm")
					case *certmanager.CertManager:
						prereqTypes = append(prereqTypes, "certmanager")
					case *ingressnginx.IngressNginx:
						prereqTypes = append(prereqTypes, "ingressnginx")
					case *opentelemetrycollector.OpenTelemetryCollectorOperator:
						prereqTypes = append(prereqTypes, "opentelemetrycollector")
					default:
						t.Fatalf("unexpected prerequisite type: %T", prereq)
					}
				}

				require.ElementsMatch(t, tc.want.expectedPrereqs, prereqTypes)
			}
		})
	}
}
