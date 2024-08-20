// Copyright 2021 Upbound Inc
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
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	metav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	metav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// ConvertToV1beta1 converts v1.Dependency types to v1beta1.Dependency types.
func ConvertToV1beta1(in metav1.Dependency) (v1beta1.Dependency, bool) {
	betaD := v1beta1.Dependency{
		Constraints: in.Version,
	}

	switch {
	case in.Provider != nil:
		betaD.Package = *in.Provider
		betaD.Type = v1beta1.ProviderPackageType

	case in.Configuration != nil:
		betaD.Package = *in.Configuration
		betaD.Type = v1beta1.ConfigurationPackageType

	case in.Function != nil:
		betaD.Package = *in.Function
		betaD.Type = v1beta1.FunctionPackageType

	default:
		return betaD, false
	}

	return betaD, true
}

// MetaConvertToV1alpha1 converts v1.Dependency types to v1alpha1.Dependency types.
func MetaConvertToV1alpha1(in metav1.Dependency) metav1alpha1.Dependency {
	alphaD := metav1alpha1.Dependency{
		Version: in.Version,
	}
	if in.Provider != nil && in.Configuration == nil {
		alphaD.Provider = in.Provider
	}

	if in.Configuration != nil && in.Provider == nil {
		alphaD.Configuration = in.Configuration
	}

	return alphaD
}

// MetaConvertToV1beta1 converts v1.Dependency types to v1beta1.Dependency types.
func MetaConvertToV1beta1(in metav1.Dependency) metav1beta1.Dependency {
	betaD := metav1beta1.Dependency{
		Version: in.Version,
	}
	if in.Provider != nil && in.Configuration == nil {
		betaD.Provider = in.Provider
	}

	if in.Configuration != nil && in.Provider == nil {
		betaD.Configuration = in.Configuration
	}

	return betaD
}
