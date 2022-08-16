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
)

var (
	// Domain specifies the demo Upbound domain.
	// NOTE(tnthornton) this field is a temporary measure that will be removed
	// when the Custom Resource exposes the status.domain field.
	Domain = "local.upbound.io"
)

// Upbound represents the Upbound CustomResource and extends an
// unstructured.Unstructured.
type Upbound struct {
	unstructured.Unstructured
}

// GetCondition returns the condition for the given xpv1.ConditionType if it
// exists, otherwise returns nil.
func (s *Upbound) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(s.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// GetDomain returns the domain field from the Upbound CustomResource.
// NOTE(tnthornton) this field does not yet exist on the CustomResource, but
// will in the near future.
func (s *Upbound) GetDomain() string {
	domain, err := fieldpath.Pave(s.Object).GetString("status.domain")
	if err != nil {
		return ""
	}
	return domain
}

// GetExternalIP returns the externalIP field from the Upbound CustomResource.
// NOTE(tnthornton) this field does not yet exist on the CustomResource, but
// will in the near future.
func (s *Upbound) GetExternalIP() string {
	ip, err := fieldpath.Pave(s.Object).GetString("status.externalIP")
	if err != nil {
		return ""
	}
	return ip
}
