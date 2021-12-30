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

package meta

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"

	"k8s.io/kube-openapi/pkg/validation/validate"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	mxpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	pyaml "github.com/upbound/up/internal/xpkg/parser/yaml"
	"github.com/upbound/up/internal/xpkg/validator"
)

var (
	dependsOnPathFmt = "spec.dependsOn[%d].%s"

	errFailedConvertToPkg = "unable to convert to package"
)

// DepManager defines the API necessary for working with the dependency manager.
type DepManager interface {
	Versions(context.Context, v1beta1.Dependency) ([]string, error)
}

// Validator defines a validator for meta files.
type Validator struct {
	p *parser.PackageParser
	// TODO(@tnthornton) move to accepting a snapshot rather than the map
	// once Snapshots are first class citizens.
	packages   map[string]*mxpkg.ParsedPackage
	validators []metaValidator
}

// New returns a new Meta validator.
func New(m DepManager, pkgs map[string]*mxpkg.ParsedPackage) (*Validator, error) {
	p, err := pyaml.New()
	if err != nil {
		return nil, err
	}

	validators := []metaValidator{
		NewTypeValidator(pkgs),
		NewVersionValidator(m),
	}

	return &Validator{
		p:          p,
		packages:   pkgs,
		validators: validators,
	}, nil
}

// Validate performs validation rules on the given data input per the rules
// defined for the Validator.
func (m *Validator) Validate(data interface{}) *validate.Result {
	pkg, err := m.Marshal(data)
	if err != nil {
		// TODO(@tnthornton) add debug logging
		return validator.Nop
	}

	errs := make([]error, 0)

	for i, d := range pkg.GetDependencies() {
		cd := manager.ConvertToV1beta1(d)
		for _, v := range m.validators {
			errs = append(errs, v.validate(i, cd))
		}
	}

	return &validate.Result{
		Errors: errs,
	}
}

// Marshal marshals the given data object into a Pkg, errors otherwise.
func (m *Validator) Marshal(data interface{}) (pkgmetav1.Pkg, error) {
	b, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}

	// convert data to a package
	ppkg, err := m.p.Parse(context.Background(), ioutil.NopCloser(bytes.NewReader(b)))
	if err != nil {
		return nil, err
	}

	if len(ppkg.GetMeta()) != 1 {
		return nil, err
	}

	pkg, ok := xpkg.TryConvertToPkg(ppkg.GetMeta()[0], &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
	if !ok {
		return nil, errors.New(errFailedConvertToPkg)
	}
	return pkg, nil
}

type metaValidator interface {
	validate(int, v1beta1.Dependency) error
}
