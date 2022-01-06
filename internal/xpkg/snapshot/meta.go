// Copyright 2022 Upbound Inc
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

package snapshot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"k8s.io/kube-openapi/pkg/validation/validate"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/semver"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	pyaml "github.com/upbound/up/internal/xpkg/parser/yaml"
	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

const (
	dependsOnPathFmt = "spec.dependsOn[%d].%s"
	versionField     = "version"

	errFailedConvertToPkg = "unable to convert to package"
	errPackageDNEFmt      = "Package %s does not exist locally. Please run `up xpkg dep` to fix."
	errVersionDENFmt      = "Version matching %s does not exist locally. Please run `up xpkg dep` to fix."
	errWrongPkgTypeFmt    = "Incorrect package type. '%s' does not match type for %s of '%s'"
)

// Validator defines a validator for meta files.
type Validator struct {
	p *parser.PackageParser
	// TODO(@tnthornton) move to accepting a snapshot rather than the map
	// once Snapshots are first class citizens.
	// packages   map[string]*mxpkg.ParsedPackage
	validators []metaValidator
}

// DefaultMetaValidators returns a new Meta validator.
func DefaultMetaValidators(s *Snapshot) (*Validator, error) {
	p, err := pyaml.New()
	if err != nil {
		return nil, err
	}

	validators := []metaValidator{
		NewTypeValidator(s),
		NewVersionValidator(s.dm),
	}

	return &Validator{
		p:          p,
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

// TypeValidator validates the dependency type matches the supplied dependency
// in a meta file.
type TypeValidator struct {
	s *Snapshot
}

// NewTypeValidator returns a new TypeValidator.
func NewTypeValidator(s *Snapshot) *TypeValidator {
	return &TypeValidator{
		s: s,
	}
}

// validate validates the dependency versions in a meta file.
func (v *TypeValidator) validate(i int, d v1beta1.Dependency) error {
	got := v.s.Package(d.Package)
	if got == nil {
		return nil
	}
	if got.Type() != d.Type {
		return &validator.MetaValidation{
			Name: fmt.Sprintf(dependsOnPathFmt, i, strings.ToLower(string(d.Type))),
			Message: fmt.Sprintf(errWrongPkgTypeFmt,
				strings.ToLower(string(d.Type)),
				strings.ToLower(d.Package),
				strings.ToLower(string(got.PType)),
			),
		}
	}
	return nil
}

// VersionValidator is used to validate the dependency versions in a meta file.
type VersionValidator struct {
	manager DepManager
}

// NewVersionValidator returns a new VersionValidator.
// NOTE(@tnthornton) NewVersionValidator needs snapshot's manager due to the
// use case where someone adds a dependency to the crossplane.yaml and we need
// to validate its existence.
func NewVersionValidator(manager DepManager) *VersionValidator {
	return &VersionValidator{
		manager: manager,
	}
}

// validate validates the dependency versions in a meta file.
func (v *VersionValidator) validate(i int, d v1beta1.Dependency) error {
	// check explicit version
	vers, err := v.manager.Versions(context.Background(), d)
	if err != nil {
		// TODO(@tnthornton) add debug logging here
		return nil
	}
	if len(vers) == 0 {
		return &validator.MetaValidation{
			Name:    fmt.Sprintf(dependsOnPathFmt, i, strings.ToLower(string(d.Type))),
			Message: fmt.Sprintf(errPackageDNEFmt, d.Package),
		}
	}
	if !versionMatch(d.Constraints, vers) {
		return &validator.MetaValidation{
			Name:    fmt.Sprintf(dependsOnPathFmt, i, versionField),
			Message: fmt.Sprintf(errVersionDENFmt, d.Constraints),
		}
	}
	return nil
}

// versionMatch returns true if the supplied constraint matches a pre-existing
// version in the supplied versions slice.
func versionMatch(constraint string, vers []string) bool {
	found := false
	// supplied version may be a semantic version constraint
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		// we're not working with a semver constraint, check for matching versions
		for _, v := range vers {
			if v == constraint {
				found = true
				break
			}
		}
	} else {
		for _, v := range vers {
			sv, err := semver.NewVersion(v)
			if err != nil {
				// invalid version, skip
				continue
			}
			if c.Check(sv) {
				// version matches semver constraint
				found = true
			}
		}
	}
	return found
}
