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
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestCheckPreReleaseConstraint(t *testing.T) {
	tests := []struct {
		name           string
		constraint     string
		version        string
		expectedResult bool
	}{
		{
			name:           "PreReleaseGreaterThanOrEqualTo",
			constraint:     ">=1.6",
			version:        "1.6.0-rc.0.75.gc383bddc",
			expectedResult: true,
		},
		{
			name:           "PreReleaseLessThanOrEqualTo",
			constraint:     "<=1.6",
			version:        "1.6.0-rc.0.75.gc383bddc",
			expectedResult: true,
		},
		{
			name:           "PreReleaseOutOfRange",
			constraint:     ">=1.8",
			version:        "1.6.0-rc.0.75.gc383bddc",
			expectedResult: false,
		},
		{
			name:           "NonPreReleaseInRange",
			constraint:     ">=1.5",
			version:        "1.5.0",
			expectedResult: true,
		},
		{
			name:           "NonPreReleaseOutOfRange",
			constraint:     ">=1.6",
			version:        "1.5.0",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraint, err := semver.NewConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("Error creating constraint: %v", err)
			}
			version, err := semver.NewVersion(tt.version)
			if err != nil {
				t.Fatalf("Error creating version: %v", err)
			}

			result := CheckPreReleaseConstraint(constraint, version)
			if result != tt.expectedResult {
				t.Errorf("CheckPreReleaseConstraint(%s, %s) = %v; want %v", tt.constraint, tt.version, result, tt.expectedResult)
			}
		})
	}
}
