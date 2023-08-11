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
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	kind = "ProviderConfig"
	// ProviderConfigHelmGVK is the GroupVersionKind used for
	// provider-helm ProviderConfig.
	ProviderConfigHelmGVK = schema.GroupVersionKind{
		Group:   "helm.crossplane.io",
		Version: "v1beta1",
		Kind:    kind,
	}
	// ProviderConfigKubernetesGVK is the GroupVersionKind used for
	// provider-kubernetes ProviderConfig.
	ProviderConfigKubernetesGVK = schema.GroupVersionKind{
		Group:   "kubernetes.crossplane.io",
		Version: "v1alpha1",
		Kind:    kind,
	}
)

// ProviderConfig represents a Crossplane ProviderConfig.
type ProviderConfig struct {
	unstructured.Unstructured
}

// GetUnstructured returns the unstructured representation of the package.
func (p *ProviderConfig) GetUnstructured() *unstructured.Unstructured {
	return &p.Unstructured
}

// SetCredentialsSource for the Provider.
func (p *ProviderConfig) SetCredentialsSource(src xpv1.CredentialsSource) {
	_ = fieldpath.Pave(p.Object).SetValue("spec.credentials.source", src)
}
