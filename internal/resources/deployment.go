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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Deployment extends an appsv1.Deployment.
type Deployment struct {
	appsv1.Deployment
}

// GetCondition returns the condition for the given DeploymentConditionType if it
// exists, otherwise returns nil.
func (s *Deployment) GetCondition(ct appsv1.DeploymentConditionType) xpv1.Condition {
	for _, c := range s.Status.Conditions {
		if c.Type == ct {
			return xpv1.Condition{
				Type:               xpv1.ConditionType(c.Type),
				Status:             c.Status,
				LastTransitionTime: c.LastTransitionTime,
				Reason:             xpv1.ConditionReason(c.Reason),
				Message:            c.Message,
			}
		}
	}

	return xpv1.Condition{Type: xpv1.ConditionType(ct), Status: corev1.ConditionUnknown}
}
