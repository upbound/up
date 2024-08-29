// Copyright 2024 The Upbound Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Project defines an Upbound Project, which can be built into a Crossplane
// Configuration.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec *ProjectSpec `json:"spec,omitempty"`
}

// ProjectSpec is the spec for a Project. Since a Project is not a Kubernetes
// resource there is no Status, only Spec.
//
// +k8s:deepcopy-gen=true
type ProjectSpec struct {
	ProjectPackageMetadata `json:",inline"`
	Repository             string                           `json:"repository"`
	Crossplane             *pkgmetav1.CrossplaneConstraints `json:"crossplane,omitempty"`
	DependsOn              []pkgmetav1.Dependency           `json:"dependsOn,omitempty"`
	Paths                  *ProjectPaths                    `json:"paths,omitempty"`
	Architectures          []string                         `json:"architectures,omitempty"`
}

// ProjectPackageMetadata holds metadata about the project, which will become
// package metadata when a project is built into a Crossplane package.
type ProjectPackageMetadata struct {
	Maintainer  string `json:"maintainer,omitempty"`
	Source      string `json:"source,omitempty"`
	License     string `json:"license,omitempty"`
	Description string `json:"description,omitempty"`
	Readme      string `json:"readme,omitempty"`
}

// ProjectPaths configures the locations of various parts of the project, for
// use at build time.
type ProjectPaths struct {
	// APIs is the directory holding the project's APIs (composite resource
	// definitions and compositions). If not specified, the builder will search
	// the whole project directory tree for APIs, excluding the functions and
	// examples directories.
	APIs string `json:"apis,omitempty"`
	// Functions is the directory holding the project's functions. If not
	// specified, it defaults to `functions/`.
	Functions string `json:"functions,omitempty"`
	// Examples is the directory holding the project's examples. If not
	// specified, it defaults to `examples/`.
	Examples string `json:"examples,omitempty"`
}

const (
	// Group is the API Group for projects.
	Group = "meta.dev.upbound.io"
	// Version is the API version for projects.
	Version = "v1alpha1"
	// GroupVersion is the GroupVersion for projects.
	GroupVersion = Group + "/" + Version
	// ProjectKind is the kind of a Project.
	ProjectKind = "Project"
)
