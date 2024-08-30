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
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"k8s.io/apimachinery/pkg/runtime"

	xpmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	xpmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/xcrd"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/parser/linter"
	"github.com/upbound/up/internal/xpkg/parser/ndjson"
	"github.com/upbound/up/internal/xpkg/parser/yaml"
	"github.com/upbound/up/internal/xpkg/scheme"
)

const (
	errFailedToParsePkgYaml         = "failed to parse package yaml"
	errLintPackage                  = "failed to lint package"
	errOpenPackageStream            = "failed to open package stream file"
	errConvertXRDs                  = "failed to convert XRD to CRD"
	errFailedToConvertMetaToPackage = "failed to convert meta to package"
	errInvalidPath                  = "invalid path provided for package lookup"
	errNotExactlyOneMeta            = "not exactly one package meta type"
)

var (
	crdGVK = schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}
)

// Marshaler represents a xpkg Marshaler
type Marshaler struct {
	yp parser.Parser
	jp JSONPackageParser
}

// NewMarshaler returns a new Marshaler
func NewMarshaler(opts ...MarshalerOption) (*Marshaler, error) {
	r := &Marshaler{}
	yp, err := yaml.New()
	if err != nil {
		return nil, err
	}

	jp, err := ndjson.New()
	if err != nil {
		return nil, err
	}

	r.yp = yp
	r.jp = jp

	for _, o := range opts {
		o(r)
	}

	return r, nil
}

// MarshalerOption modifies the xpkg Marshaler
type MarshalerOption func(*Marshaler)

// WithYamlParser modifies the Marshaler by setting the supplied PackageParser as
// the Resolver's parser.
func WithYamlParser(p parser.Parser) MarshalerOption {
	return func(r *Marshaler) {
		r.yp = p
	}
}

// WithJSONParser modifies the Marshaler by setting the supplied PackageParser as
// the Resolver's parser.
func WithJSONParser(p JSONPackageParser) MarshalerOption {
	return func(r *Marshaler) {
		r.jp = p
	}
}

// FromImage takes a xpkg.Image and returns a ParsedPackage for consumption by
// upstream callers.
func (r *Marshaler) FromImage(i xpkg.Image) (*ParsedPackage, error) {
	reader := mutate.Extract(i.Image)
	fs := tarfs.New(tar.NewReader(reader))
	pkgYaml, err := fs.Open(xpkg.StreamFile)
	if err != nil {
		return nil, errors.Wrap(err, errOpenPackageStream)
	}

	pkg, err := r.parseYaml(pkgYaml)
	if err != nil {
		return nil, err
	}

	pkg = applyImageMeta(i.Meta, pkg)

	if pkg, err = convertXRD2CRD(pkg); err != nil {
		return nil, errors.Wrap(err, errConvertXRDs)
	}

	return finalizePkg(pkg)
}

// FromDir takes an afero.Fs and a path to a directory and returns a
// ParsedPackage based on the directories contents for consumption by upstream
// callers.
func (r *Marshaler) FromDir(fs afero.Fs, path string) (*ParsedPackage, error) {
	parts := strings.Split(path, "@")
	if len(parts) != 2 {
		return nil, errors.New(errInvalidPath)
	}

	pkgJSON, err := fs.Open(filepath.Join(path, xpkg.JSONStreamFile))
	if err != nil {
		return nil, err
	}

	pkg, err := r.parseNDJSON(pkgJSON)
	if err != nil {
		return nil, err
	}

	return finalizePkg(pkg)
}

// parseYaml parses the
func (r *Marshaler) parseYaml(reader io.ReadCloser) (*ParsedPackage, error) {
	pkg, err := r.yp.Parse(context.Background(), reader)
	if err != nil {
		return nil, errors.Wrap(err, errFailedToParsePkgYaml)
	}
	return processPackage(pkg)
}

func processPackage(pkg linter.Package) (*ParsedPackage, error) {
	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, errors.New(errNotExactlyOneMeta)
	}

	meta := metas[0]
	var linter linter.Linter
	var pkgType v1beta1.PackageType
	switch meta.GetObjectKind().GroupVersionKind().Kind {
	case xpmetav1.ConfigurationKind:
		linter = xpkg.NewConfigurationLinter()
		pkgType = v1beta1.ConfigurationPackageType
	case xpmetav1.ProviderKind:
		linter = xpkg.NewProviderLinter()
		pkgType = v1beta1.ProviderPackageType
	case xpmetav1beta1.FunctionKind:
		linter = xpkg.NewFunctionLinter()
		pkgType = v1beta1.FunctionPackageType
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, errors.Wrap(err, errLintPackage)
	}

	return &ParsedPackage{
		MetaObj: meta,
		Objs:    pkg.GetObjects(),
		PType:   pkgType,
	}, nil
}

func (r *Marshaler) parseNDJSON(reader io.ReadCloser) (*ParsedPackage, error) {
	pkg, err := r.jp.Parse(context.Background(), reader)
	if err != nil {
		return nil, errors.Wrap(err, errFailedToParsePkgYaml)
	}

	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, errors.New(errNotExactlyOneMeta)
	}

	meta := metas[0]

	// Check if the meta kind is ConfigurationKind
	if meta.GetObjectKind().GroupVersionKind().Kind == xpmetav1.ConfigurationKind {
		filteredObjects := []runtime.Object{}
		for _, obj := range pkg.GetObjects() {
			// Only include objects of type CompositeResourceDefinition or Composition
			if _, isXRD := obj.(*v1.CompositeResourceDefinition); isXRD {
				filteredObjects = append(filteredObjects, obj)
			} else if _, isComposition := obj.(*v1.Composition); isComposition {
				filteredObjects = append(filteredObjects, obj)
			}
		}
		// Replace pkg.objects with the filtered list
		pkg.SetObjects(filteredObjects)
	}

	p, err := processPackage(pkg)
	if err != nil {
		return nil, err
	}

	return applyImageMeta(pkg.GetImageMeta(), p), nil
}

func applyImageMeta(m xpkg.ImageMeta, pkg *ParsedPackage) *ParsedPackage {
	pkg.DepName = m.Repo
	pkg.Reg = m.Registry
	pkg.SHA = m.Digest
	pkg.Ver = m.Version

	return pkg
}

func convertXRD2CRD(pkg *ParsedPackage) (*ParsedPackage, error) {
	for _, obj := range pkg.Objects() {
		if obj.GetObjectKind().GroupVersionKind().Kind == "CompositeResourceDefinition" {
			xrd := obj.(*v1.CompositeResourceDefinition)

			crd, err := xcrd.ForCompositeResource(obj.(*v1.CompositeResourceDefinition))
			if err != nil {
				return nil, errors.Wrapf(err, "cannot derive composite CRD from XRD %q", xrd.GetName())
			}
			crd.SetGroupVersionKind(crdGVK)
			pkg.Objs = append(pkg.Objs, crd)

			if xrd.Spec.ClaimNames != nil {
				claimCrd, err := xcrd.ForCompositeResourceClaim(obj.(*v1.CompositeResourceDefinition))
				if err != nil {
					return nil, errors.Wrapf(err, "cannot derive claim CRD from XRD %q", xrd.GetName())
				}
				claimCrd.SetGroupVersionKind(crdGVK)
				pkg.Objs = append(pkg.Objs, claimCrd)
			}
		}
	}

	return pkg, nil
}

func finalizePkg(pkg *ParsedPackage) (*ParsedPackage, error) { // nolint:gocyclo
	deps, err := determineDeps(pkg.MetaObj)
	if err != nil {
		return nil, err
	}

	pkg.Deps = deps

	return pkg, nil
}

func determineDeps(o runtime.Object) ([]v1beta1.Dependency, error) {
	pkg, ok := scheme.TryConvertToPkg(o, &xpmetav1.Provider{}, &xpmetav1.Configuration{}, &xpmetav1.Function{})
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

	if in.Provider != nil {
		betaD.Package = *in.Provider
		betaD.Type = v1beta1.ProviderPackageType
	}

	if in.Configuration != nil {
		betaD.Package = *in.Configuration
		betaD.Type = v1beta1.ConfigurationPackageType
	}

	if in.Function != nil {
		betaD.Package = *in.Function
		betaD.Type = v1beta1.FunctionPackageType
	}

	return betaD
}
