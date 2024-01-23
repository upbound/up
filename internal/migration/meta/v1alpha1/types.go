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

package v1alpha1

import (
	"time"
)

// Directory structure for export:
// export.yaml (with ExportMeta below)
// <groupResource>/<cluster or namespace>/<?namespace>/<name>.yaml
// <groupResource>/metadata.yaml (with TypeMeta below)

// TypeMeta is the metadata for a given resource type.
type TypeMeta struct {
	// Categories are the categories of the resource type.
	Categories []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	// WithStatusSubresource indicates whether the resource type has a status subresource.
	WithStatusSubresource bool `json:"withStatusSubresource,omitempty" yaml:"withStatusSubresource,omitempty"`
}

// ExportStats are the statistics about the exported resources.
type ExportStats struct {
	// Total is the total number of resources exported.
	Total int `json:"total,omitempty" yaml:"total,omitempty"`
	// NativeResources keeps track of the number of native resources exported per GVK.
	NativeResources map[string]int `json:"nativeResources,omitempty" yaml:"nativeResources,omitempty"`
	// CustomResources keeps track of the number of custom resources exported per GVK.
	CustomResources map[string]int `json:"customResources,omitempty" yaml:"customResources,omitempty"`
}

// CrossplaneInfo is the information about the Crossplane instance on the exported control plane.
type CrossplaneInfo struct {
	// Distribution is the distribution of Crossplane, e.g. "crossplane" or "universal-crossplane".
	Distribution string `json:"distribution,omitempty" yaml:"distribution,omitempty"`
	// Namespace is the namespace in which Crossplane is installed.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Version is the version of Crossplane.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// FeatureFlags are the feature flags enabled in Crossplane.
	FeatureFlags []string `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

// ExportOptions are the options used to create the export.
type ExportOptions struct {
	// IncludedNamespaces are the namespaces included in the export.
	IncludedNamespaces []string `json:"includedNamespaces,omitempty" yaml:"includedNamespaces,omitempty"`
	// ExcludedNamespaces are the namespaces excluded from the export.
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty" yaml:"excludedNamespaces,omitempty"`
	// IncludedResources are the resources included in the export.
	IncludedResources []string `json:"includedResources,omitempty" yaml:"includedResources,omitempty"`
	// ExcludedResources are the resources excluded from the export.
	ExcludedResources []string `json:"excludedResources,omitempty" yaml:"excludedResources,omitempty"`
	// PausedBeforeExport stores whether the resources were paused before the export.
	PausedBeforeExport bool `json:"pausedBeforeExport,omitempty" yaml:"pausedBeforeExport,omitempty"`
}

// ExportMeta is the top level metadata for an export.
type ExportMeta struct {
	// Version is the API version of the export. This will be used to determine
	// compatibility with the importer once we evolve the export format.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// ExportedAt is the time at which the export was created.
	ExportedAt time.Time `json:"exportedAt,omitempty" yaml:"exportedAt,omitempty"`
	// Options are the options used to create the export.
	Options ExportOptions `json:"options,omitempty" yaml:"options,omitempty"`
	// Crossplane is the information about the Crossplane instance on the exported control plane.
	Crossplane CrossplaneInfo `json:"crossplane,omitempty" yaml:"crossplane,omitempty"`
	// Stats are the statistics about the exported resources.
	Stats ExportStats `json:"stats,omitempty" yaml:"stats,omitempty"`
}
