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
	Categories            []string `json:"categories,omitempty"`
	WithStatusSubresource bool     `json:"withStatusSubresource,omitempty"`
}

type ExportStats struct {
	Total           int            `json:"total,omitempty"`
	NativeResources map[string]int `json:"nativeResources,omitempty"`
	CustomResources map[string]int `json:"customResources,omitempty"`
}

type CrossplaneInfo struct {
	Distribution string   `json:"distribution,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	Version      string   `json:"version,omitempty"`
	FeatureFlags []string `json:"featureFlags,omitempty"`
}

type ExportOptions struct {
	IncludedNamespaces []string
	ExcludedNamespaces []string

	IncludedResources []string
	ExcludedResources []string
}

type ExportMeta struct {
	Version    string         `json:"version,omitempty"`
	ExportedAt time.Time      `json:"exportedAt,omitempty"`
	Options    ExportOptions  `json:"options,omitempty"`
	Crossplane CrossplaneInfo `json:"crossplane,omitempty"`
	Stats      ExportStats    `json:"stats,omitempty"`
}
