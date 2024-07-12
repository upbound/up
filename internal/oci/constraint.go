// Copyright 2024 Upbound Inc
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

package oci

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// CheckPreReleaseConstraint checks whether a given version, stripped of its pre-release suffix
func CheckPreReleaseConstraint(constraint *semver.Constraints, version *semver.Version) bool {
	// Create a new version without the pre-release
	baseVersionStr := fmt.Sprintf("%d.%d.%d", version.Major(), version.Minor(), version.Patch())
	baseVersion, err := semver.NewVersion(baseVersionStr)
	if err != nil {
		return false
	}
	// Check the base version against the constraint
	return constraint.Check(baseVersion)
}
