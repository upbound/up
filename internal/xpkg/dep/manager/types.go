// Copyright 2021 Upbound Inc
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

package manager

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
)

// Cache --
type Cache interface {
	Get(v1beta1.Dependency) (*xpkg.ParsedPackage, error)
	Store(v1beta1.Dependency, *xpkg.ParsedPackage) error
}

// ImageResolver --
type ImageResolver interface {
	ResolveDigest(context.Context, v1beta1.Dependency) (string, error)
	ResolveImage(context.Context, v1beta1.Dependency) (string, v1.Image, error)
	ResolveTag(context.Context, v1beta1.Dependency) (string, error)
}

// XpkgMarshaler --
type XpkgMarshaler interface {
	FromImage(string, string, v1.Image) (*xpkg.ParsedPackage, error)
	FromDir(afero.Fs, string) (*xpkg.ParsedPackage, error)
}
