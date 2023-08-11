// Copyright 2022 Upbound Inc
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
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	xppkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Package represents a Crossplane Package.
type Package struct {
	unstructured.Unstructured
}

// GetUnstructured returns the unstructured representation of the package.
func (p *Package) GetUnstructured() *unstructured.Unstructured {
	return &p.Unstructured
}

// GetInstalled checks whether a package is installed. If installation status
// cannot be determined, false is always returned.
func (p *Package) GetInstalled() bool {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(p.Object).GetValueInto("status", &conditioned); err != nil {
		return false
	}
	return resource.IsConditionTrue(conditioned.GetCondition("Installed"))
}

// GetHealthy checks whether a package is healhty. If health cannot be
// determined, false is always returned.
func (p *Package) GetHealthy() bool {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(p.Object).GetValueInto("status", &conditioned); err != nil {
		return false
	}
	return resource.IsConditionTrue(conditioned.GetCondition("Healthy"))
}

// SetPackage sets the package reference.
func (p *Package) SetPackage(pkg string) {
	_ = fieldpath.Pave(p.Object).SetValue("spec.package", pkg)
}

// SetControllerConfigRef sets the controllerConfigRef on the package.
func (p *Package) SetControllerConfigRef(ref xppkgv1.ControllerConfigReference) {
	_ = fieldpath.Pave(p.Object).SetValue("spec.controllerConfigRef", ref)
}
