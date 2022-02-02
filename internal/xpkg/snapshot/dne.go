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

package snapshot

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

const (
	warnNoDefinitionFound = "no definition found for resource"
)

// gvkDNEWarning returns a Validation indicating warning that a validator
// could not be found for the given gvk. Location is provided to indicate
// where the warning should be surfaced.
func gvkDNEWarning(gvk schema.GroupVersionKind, location string) []error {
	return []error{
		&validator.Validation{
			TypeCode: validator.WarningTypeCode,
			Message:  fmt.Sprintf(errFmt, warnNoDefinitionFound, gvk),
			Name:     location,
		},
	}
}
