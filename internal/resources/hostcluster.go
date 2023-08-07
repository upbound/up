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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HostCluster represents the HostCluster CustomResource and extends an
// unstructured.Unstructured.
type HostCluster struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (h *HostCluster) GetUnstructured() *unstructured.Unstructured {
	return &h.Unstructured
}

// GetCondition returns the condition for the given xpv1.ConditionType if it
// exists, otherwise returns nil.
func (h *HostCluster) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(h.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetCompositionSelector of this composite resource claim.
func (h *HostCluster) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(h.Object).SetValue("spec.compositionSelector", sel)
}
