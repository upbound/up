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

package resources

import (
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ControllerConfigGRV is the GroupVersionResource used for
	// the Crossplane ControllerConfig.
	ControllerConfigGRV = schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1alpha1",
		Resource: "controllerconfigs",
	}
)

// ControllerConfig represents a Crossplane ControllerConfig.
type ControllerConfig struct {
	unstructured.Unstructured
}

// GetUnstructured returns the unstructured representation of the package.
func (c *ControllerConfig) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// SetServiceAccountName for the ControllerConfig.
func (c *ControllerConfig) SetServiceAccountName(name string) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.serviceAccountName", name)
}
