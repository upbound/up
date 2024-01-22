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

type TypeMeta struct {
	Categories            []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	WithStatusSubresource bool     `json:"withStatusSubresource,omitempty" yaml:"withStatusSubresource,omitempty"`
}

type ExportStats struct {
	Total           int            `json:"total,omitempty" yaml:"total,omitempty"`
	NativeResources map[string]int `json:"nativeResources,omitempty" yaml:"nativeResources,omitempty"`
	CustomResources map[string]int `json:"customResources,omitempty" yaml:"customResources,omitempty"`
}

type CrossplaneInfo struct {
	Distribution string   `json:"distribution,omitempty" yaml:"distribution,omitempty"`
	Namespace    string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Version      string   `json:"version,omitempty" yaml:"version,omitempty"`
	FeatureFlags []string `json:"featureFlags,omitempty" yaml:"featureFlags,omitempty"`
}

type ExportOptions struct {
	IncludedNamespaces []string `json:"includedNamespaces,omitempty" yaml:"includedNamespaces,omitempty"`
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty" yaml:"excludedNamespaces,omitempty"`

	IncludedResources []string `json:"includedResources,omitempty" yaml:"includedResources,omitempty"`
	ExcludedResources []string `json:"excludedResources,omitempty" yaml:"excludedResources,omitempty"`
}

type ExportMeta struct {
	Version    string         `json:"version,omitempty" yaml:"version,omitempty"`
	ExportedAt time.Time      `json:"exportedAt,omitempty" yaml:"exportedAt,omitempty"`
	Options    ExportOptions  `json:"options,omitempty" yaml:"options,omitempty"`
	Crossplane CrossplaneInfo `json:"crossplane,omitempty" yaml:"crossplane,omitempty"`
	Stats      ExportStats    `json:"stats,omitempty" yaml:"stats,omitempty"`
}
