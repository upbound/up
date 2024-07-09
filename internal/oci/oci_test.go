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
	"context"
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/google/go-containerregistry/pkg/crane"
)

// TestGetArtifactName tests the GetArtifactName function
func TestGetArtifactName(t *testing.T) {
	tests := []struct {
		name     string
		artifact string
		expected string
	}{
		{
			name:     "Basic Case",
			artifact: "oci://xpkg.upbound.io/spaces-artifacts/spaces:1.0.0",
			expected: "spaces-1.0.0",
		},
		{
			name:     "No Version",
			artifact: "xpkg.upbound.io/spaces-artifacts/spaces",
			expected: "spaces",
		},
		{
			name:     "Multiple Colons",
			artifact: "oci://xpkg.upbound.io/spaces-artifacts/spaces:1.0.0:latest",
			expected: "spaces-1.0.0-latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetArtifactName(tt.artifact)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRemoveDomainAndOrg tests the RemoveDomainAndOrg function
func TestRemoveDomainAndOrg(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		expected string
	}{
		{
			name:     "Basic Case",
			src:      "xpkg.upbound.io/org/repo",
			expected: "repo",
		},
		{
			name:     "Missing Parts",
			src:      "repo",
			expected: "repo",
		},
		{
			name:     "Only Domain",
			src:      "xpkg.upbound.io/repo",
			expected: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveDomainAndOrg(tt.src)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTagExists tests the TagExists function
func TestTagExists(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		version  string
		expected bool
	}{
		{
			name:     "Tag Exists",
			tags:     []string{"1.0.0", "1.1.0", "1.2.0"},
			version:  "1.1.0",
			expected: true,
		},
		{
			name:     "Tag Does Not Exist",
			tags:     []string{"1.0.0", "1.1.0", "1.2.0"},
			version:  "2.0.0",
			expected: false,
		},
		{
			name:     "Empty Tags",
			tags:     []string{},
			version:  "1.0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TagExists(tt.tags, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestListTags tests the ListTags function
func TestListTags(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		mockTags   []string
		mockError  error
		expected   []string
		expectErr  bool
	}{
		{
			name:       "Successful Tag Listing",
			repository: "spaces",
			mockTags:   []string{"1.0.0", "1.1.0", "1.2.0"},
			mockError:  nil,
			expected:   []string{"1.0.0", "1.1.0", "1.2.0"},
			expectErr:  false,
		},
		{
			name:       "Error in Tag Listing",
			repository: "spaces",
			mockTags:   nil,
			mockError:  errors.New("failed to list tags"),
			expected:   nil,
			expectErr:  true,
		},
	}

	originalListTags := DefaultListTags
	defer func() { DefaultListTags = originalListTags }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DefaultListTags = func(repo string, options ...crane.Option) ([]string, error) {
				if repo == tt.repository {
					return tt.mockTags, tt.mockError
				}
				return nil, nil
			}

			result, err := ListTags(context.Background(), tt.repository)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
