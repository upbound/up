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

package xpkg

import (
	"archive/tar"
	"context"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
	"github.com/spf13/afero/tarfs"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	xpmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
)

const (
	errFailedToParsePkgYaml         = "failed to parse package yaml"
	errLintPackage                  = "failed to lint package"
	errOpenPackageStream            = "failed to open package stream file"
	errFaileToAcquireDigest         = "failed to pull digest from image"
	errFailedToConvertMetaToPackage = "failed to convert meta to package"

	errNotExactlyOneMeta = "not exactly one package meta type"
)

// Resolver represents a xpkg Resolver
type Resolver struct {
	p *parser.PackageParser
}

// ParsedPackage represents an xpkg that has been parsed from a v1.Image
type ParsedPackage struct {
	// The SHA digest corresponding to the package.
	digest string
	// The package dependencies derived from .Spec.DependsOn.
	deps []v1beta1.Dependency
	// The meta file that corresponds to the package.
	meta runtime.Object
	// The N corresponding objects (CRDs, XRDs, Compositions) depending on the package type.
	objects []runtime.Object
	// The type of Package.
	ptype v1beta1.PackageType
	// The container registry.
	reg string
	// The resolved version, e.g. v0.20.0
	ver string
}

// Digest returns the package's digest derived from the package image.
func (p *ParsedPackage) Digest() string {
	return p.digest
}

// Dependencies returns the package's dependencies.
func (p *ParsedPackage) Dependencies() []v1beta1.Dependency {
	return p.deps
}

// Meta returns the runtime.Object corresponding to the meta file.
func (p *ParsedPackage) Meta() runtime.Object {
	return p.meta
}

// Objects returns the slice of runtime.Objects corresponding to CRDs, XRDs, and
// Compositions contained in the package.
func (p *ParsedPackage) Objects() []runtime.Object {
	return p.objects
}

// Type returns the package's type.
func (p *ParsedPackage) Type() v1beta1.PackageType {
	return p.ptype
}

// Registry returns the registry path where the package image is located.
// e.g. index.docker.io/crossplane/provider-aws
func (p *ParsedPackage) Registry() string {
	return p.reg
}

// Version returns the version for the package image.
// e.g. v0.20.0
func (p *ParsedPackage) Version() string {
	return p.ver
}

// NewResolver returns a new Resolver
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{}

	for _, o := range opts {
		o(r)
	}

	return r
}

// ResolverOption modifies the xpkg Resolver
type ResolverOption func(*Resolver)

// WithParser modifies the Resolver by setting the supplied PackageParser as
// the Resolver's parser.
func WithParser(p *parser.PackageParser) ResolverOption {
	return func(r *Resolver) {
		r.p = p
	}
}

// FromImage takes a registry and version string and their corresponding v1.Image and
// returns a ParsedPackage for consumption by upstream callers.
func (r *Resolver) FromImage(reg, ver string, i v1.Image) (*ParsedPackage, error) {
	digest, err := i.Digest()
	if err != nil {
		return nil, errors.Wrap(err, errFaileToAcquireDigest)
	}

	reader := mutate.Extract(i)
	fs := tarfs.New(tar.NewReader(reader))
	pkgYaml, err := fs.Open(xpkg.StreamFile)
	if err != nil {
		return nil, errors.Wrap(err, errOpenPackageStream)
	}

	pkg, err := r.parse(pkgYaml)
	if err != nil {
		return nil, err
	}

	deps, err := determineDeps(pkg.meta)
	if err != nil {
		return nil, err
	}

	pkg.deps = deps
	pkg.digest = digest.String()
	pkg.reg = reg
	pkg.ver = ver

	return pkg, nil
}

// FromDir takes a path to a directory and returns a ParsedPackage based on the
// directories contents for consumption by upstream callers.
func (r *Resolver) FromDir(path string) (*ParsedPackage, error) {
	return nil, nil
}

func (r *Resolver) parse(reader io.ReadCloser) (*ParsedPackage, error) {
	var pkgType v1beta1.PackageType
	// parse package.yaml
	pkg, err := r.p.Parse(context.Background(), reader)
	if err != nil {
		return nil, errors.Wrap(err, errFailedToParsePkgYaml)
	}

	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, errors.New(errNotExactlyOneMeta)
	}

	meta := metas[0]
	var linter parser.Linter
	if meta.GetObjectKind().GroupVersionKind().Kind == xpmetav1.ConfigurationKind {
		linter = xpkg.NewConfigurationLinter()
		pkgType = v1beta1.ConfigurationPackageType
	} else {
		linter = xpkg.NewProviderLinter()
		pkgType = v1beta1.ProviderPackageType
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, errors.Wrap(err, errLintPackage)
	}

	p := &ParsedPackage{
		meta:    meta,
		objects: pkg.GetObjects(),
		ptype:   pkgType,
	}

	return p, nil
}

func determineDeps(o runtime.Object) ([]v1beta1.Dependency, error) {
	pkg, ok := xpkg.TryConvertToPkg(o, &xpmetav1.Provider{}, &xpmetav1.Configuration{})
	if !ok {
		return nil, errors.New(errFailedToConvertMetaToPackage)
	}

	out := make([]v1beta1.Dependency, len(pkg.GetDependencies()))
	for i, d := range pkg.GetDependencies() {
		out[i] = convertToV1beta1(d)
	}

	return out, nil
}

func convertToV1beta1(in xpmetav1.Dependency) v1beta1.Dependency {
	betaD := v1beta1.Dependency{
		Constraints: in.Version,
	}
	if in.Provider != nil && in.Configuration == nil {
		betaD.Package = *in.Provider
		betaD.Type = v1beta1.ProviderPackageType
	}

	if in.Configuration != nil && in.Provider == nil {
		betaD.Package = *in.Configuration
		betaD.Type = v1beta1.ConfigurationPackageType
	}

	return betaD
}
