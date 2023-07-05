/*
Copyright 2021 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package composite

import (
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// AsCompositionConnectionDetail translates a composition revision's connection
// detail to a composition connection detail.
func AsCompositionConnectionDetail(rcd v1.ConnectionDetail) v1.ConnectionDetail {
	return v1.ConnectionDetail{
		Name: rcd.Name,
		Type: func() *v1.ConnectionDetailType {
			if rcd.Type == nil {
				return nil
			}
			t := v1.ConnectionDetailType(*rcd.Type)
			return &t
		}(),
		FromConnectionSecretKey: rcd.FromConnectionSecretKey,
		FromFieldPath:           rcd.FromFieldPath,
		Value:                   rcd.Value,
	}
}

// AsCompositionReadinessCheck translates a composition revision's readiness
// check to a composition readiness check.
func AsCompositionReadinessCheck(rrc v1.ReadinessCheck) v1.ReadinessCheck {
	return v1.ReadinessCheck{
		Type:         v1.ReadinessCheckType(rrc.Type),
		FieldPath:    rrc.FieldPath,
		MatchString:  rrc.MatchString,
		MatchInteger: rrc.MatchInteger,
	}
}
