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
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
)

// ListTagsFunc is a type for the ListTags function
type ListTagsFunc func(repo string, options ...crane.Option) ([]string, error)

// DefaultListTags is the default implementation of ListTagsFunc
var DefaultListTags ListTagsFunc = crane.ListTags

// GetArtifactName extracts the artifact name from the chart reference and replaces ':' with '-'
func GetArtifactName(artifact string) string {
	parts := strings.Split(artifact, "/")
	artifactPathName := parts[len(parts)-1]
	return strings.ReplaceAll(artifactPathName, ":", "-")
}

// RemoveDomainAndOrg removes the domain and organization from the repository URL
func RemoveDomainAndOrg(src string) string {
	parts := strings.SplitN(src, "/", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	if len(parts) == 2 {
		return parts[1]
	}
	return src
}

// TagExists checks if a specific version exists in the list of tags
func TagExists(tags []string, version string) bool {
	for _, tag := range tags {
		if tag == version {
			return true
		}
	}
	return false
}

// ListTags lists the tags for a given repository
func ListTags(ctx context.Context, repository string) ([]string, error) {
	return DefaultListTags(repository)
}
