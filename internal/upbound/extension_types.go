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

package upbound

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
)

// DisconnectedConfiguration is the configuration for a disconnected space
type DisconnectedConfiguration struct {
	HubContext string `json:"hubContext"`
}

// CloudConfiguration is the configuration of a cloud space
type CloudConfiguration struct {
	Organization string `json:"organization"`
}

// SpaceExtensionSpec is the spec of SpaceExtension
//
// +k8s:deepcopy-gen=true
type SpaceExtensionSpec struct {
	Disconnected *DisconnectedConfiguration `json:"disconnected,omitempty"`
	Cloud        *CloudConfiguration        `json:"cloud,omitempty"`
}

// SpaceExtension is a kubeconfig context extension that defines metadata about
// a space context
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SpaceExtension struct {
	metav1.TypeMeta `json:",inline"`

	Spec *SpaceExtensionSpec `json:"spec,omitempty"`
}

var (
	// SpaceExtensionKind is kind of SpaceExtension
	SpaceExtensionKind = reflect.TypeOf(SpaceExtension{}).Name()
)

func NewCloudV1Alpha1SpaceExtension(org string) *SpaceExtension {
	return &SpaceExtension{
		TypeMeta: metav1.TypeMeta{
			Kind:       SpaceExtensionKind,
			APIVersion: upboundv1alpha1.SchemeGroupVersion.String(),
		},
		Spec: &SpaceExtensionSpec{
			Cloud: &CloudConfiguration{
				Organization: org,
			},
		},
	}
}

func NewDisconnectedV1Alpha1SpaceExtension(hubContext string) *SpaceExtension {
	return &SpaceExtension{
		TypeMeta: metav1.TypeMeta{
			Kind:       SpaceExtensionKind,
			APIVersion: "spaces.upbound.io/v1alpha1",
		},
		Spec: &SpaceExtensionSpec{
			Disconnected: &DisconnectedConfiguration{
				HubContext: hubContext,
			},
		},
	}
}
