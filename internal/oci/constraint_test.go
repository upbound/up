// constraint_test.go

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
