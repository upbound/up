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

package manager

import (
	"reflect"
	"testing"

	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	metav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	metav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"k8s.io/utils/ptr"
)

func TestConvertToV1beta1(t *testing.T) {
	tests := []struct {
		name           string
		input          metav1.Dependency
		expectedOutput v1beta1.Dependency
		expectedBool   bool
	}{
		{
			name: "Provider set",
			input: metav1.Dependency{
				Version:  "v1.0.0",
				Provider: ptr.To("provider-example"),
			},
			expectedOutput: v1beta1.Dependency{
				Constraints: "v1.0.0",
				Package:     "provider-example",
				Type:        v1beta1.ProviderPackageType,
			},
			expectedBool: true,
		},
		{
			name: "Configuration set",
			input: metav1.Dependency{
				Version:       "v1.0.0",
				Configuration: ptr.To("configuration-example"),
			},
			expectedOutput: v1beta1.Dependency{
				Constraints: "v1.0.0",
				Package:     "configuration-example",
				Type:        v1beta1.ConfigurationPackageType,
			},
			expectedBool: true,
		},
		{
			name: "Function set",
			input: metav1.Dependency{
				Version:  "v1.0.0",
				Function: ptr.To("function-example"),
			},
			expectedOutput: v1beta1.Dependency{
				Constraints: "v1.0.0",
				Package:     "function-example",
				Type:        v1beta1.FunctionPackageType,
			},
			expectedBool: true,
		},
		{
			name: "No Provider, Configuration, or Function set",
			input: metav1.Dependency{
				Version: "v1.0.0",
			},
			expectedOutput: v1beta1.Dependency{
				Constraints: "v1.0.0",
			},
			expectedBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOutput, gotBool := ConvertToV1beta1(tt.input)
			if !reflect.DeepEqual(gotOutput, tt.expectedOutput) || gotBool != tt.expectedBool {
				t.Errorf("ConvertToV1beta1() = (%v, %v), want (%v, %v)", gotOutput, gotBool, tt.expectedOutput, tt.expectedBool)
			}
		})
	}
}

func TestMetaConvertToV1alpha1(t *testing.T) {
	tests := []struct {
		name     string
		input    metav1.Dependency
		expected metav1alpha1.Dependency
	}{
		{
			name: "Provider set, Configuration nil",
			input: metav1.Dependency{
				Version:  "v1.0.0",
				Provider: ptr.To("provider-example"),
			},
			expected: metav1alpha1.Dependency{
				Version:  "v1.0.0",
				Provider: ptr.To("provider-example"),
			},
		},
		{
			name: "Configuration set, Provider nil",
			input: metav1.Dependency{
				Version:       "v1.0.0",
				Configuration: ptr.To("configuration-example"),
			},
			expected: metav1alpha1.Dependency{
				Version:       "v1.0.0",
				Configuration: ptr.To("configuration-example"),
			},
		},
		{
			name: "Both Provider and Configuration nil",
			input: metav1.Dependency{
				Version: "v1.0.0",
			},
			expected: metav1alpha1.Dependency{
				Version: "v1.0.0",
			},
		},
		{
			name: "Both Provider and Configuration set",
			input: metav1.Dependency{
				Version:       "v1.0.0",
				Provider:      ptr.To("provider-example"),
				Configuration: ptr.To("configuration-example"),
			},
			expected: metav1alpha1.Dependency{
				Version: "v1.0.0",
				// No Provider or Configuration should be set since both are provided in input
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetaConvertToV1alpha1(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MetaConvertToV1alpha1() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMetaConvertToV1beta1(t *testing.T) {
	tests := []struct {
		name     string
		input    metav1.Dependency
		expected metav1beta1.Dependency
	}{
		{
			name: "Provider set, Configuration nil",
			input: metav1.Dependency{
				Version:  "v1.0.0",
				Provider: ptr.To("provider-example"),
			},
			expected: metav1beta1.Dependency{
				Version:  "v1.0.0",
				Provider: ptr.To("provider-example"),
			},
		},
		{
			name: "Configuration set, Provider nil",
			input: metav1.Dependency{
				Version:       "v1.0.0",
				Configuration: ptr.To("configuration-example"),
			},
			expected: metav1beta1.Dependency{
				Version:       "v1.0.0",
				Configuration: ptr.To("configuration-example"),
			},
		},
		{
			name: "Both Provider and Configuration nil",
			input: metav1.Dependency{
				Version: "v1.0.0",
			},
			expected: metav1beta1.Dependency{
				Version: "v1.0.0",
			},
		},
		{
			name: "Both Provider and Configuration set",
			input: metav1.Dependency{
				Version:       "v1.0.0",
				Provider:      ptr.To("provider-example"),
				Configuration: ptr.To("configuration-example"),
			},
			expected: metav1beta1.Dependency{
				Version: "v1.0.0",
				// No Provider or Configuration should be set since both are provided in input
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetaConvertToV1beta1(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MetaConvertToV1beta1() = %v, want %v", got, tt.expected)
			}
		})
	}
}
