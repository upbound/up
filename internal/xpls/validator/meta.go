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

package validator

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"k8s.io/kube-openapi/pkg/validation/validate"
	"sigs.k8s.io/yaml"

	"github.com/Masterminds/semver"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"

	"github.com/upbound/up/internal/xpkg"
	"github.com/upbound/up/internal/xpkg/dep/manager"
	xpkgparser "github.com/upbound/up/internal/xpkg/parser"
)

const (
	dependsOnPathFmt = "spec.dependsOn[%d].%s"
	versionField     = "version"

	errPackageDNEFmt = "package %s does not exist locally. Please run `up xpkg dep` to fix."
	errVersionDENFmt = "version matching %s does not exist locally. Please run `up xpkg dep` to fix."
)

// Meta defines a validator for meta files.
type Meta struct {
	manager *manager.Manager
	p       *parser.PackageParser
}

// NewMeta constructs a meta validator.
func NewMeta(m *manager.Manager) (*Meta, error) {

	p, err := xpkgparser.New()
	if err != nil {
		return nil, err
	}

	return &Meta{
		manager: m,
		p:       p,
	}, nil
}

// Validate validates the given data representing a meta file.
func (m *Meta) Validate(data interface{}) *validate.Result { // nolint:gocyclo
	noop := &validate.Result{}

	b, err := yaml.Marshal(data)
	if err != nil {
		return noop
	}

	// convert data to a package
	ppkg, err := m.p.Parse(context.Background(), ioutil.NopCloser(bytes.NewReader(b)))
	if err != nil {
		return noop
	}

	if len(ppkg.GetMeta()) != 1 {
		return noop
	}

	pkg, ok := xpkg.TryConvertToPkg(ppkg.GetMeta()[0], &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
	if !ok {
		return noop
	}

	errs := make([]error, 0)
	for i, d := range pkg.GetDependencies() {
		cd := manager.ConvertToV1beta1(d)
		// check explicit version
		vers, err := m.manager.Versions(context.Background(), cd)
		if err != nil {
			// TODO(@tnthornton) add debug logging here
			continue
		}
		if len(vers) == 0 {
			errs = append(errs, &MetaValidaton{
				Name:    fmt.Sprintf(dependsOnPathFmt, i, strings.ToLower(string(cd.Type))),
				message: fmt.Sprintf(errPackageDNEFmt, cd.Package),
			})
			continue
		}
		if !versionMatch(cd.Constraints, vers) {
			errs = append(errs, &MetaValidaton{
				Name:    fmt.Sprintf(dependsOnPathFmt, i, versionField),
				message: fmt.Sprintf(errVersionDENFmt, cd.Constraints),
			})
		}
	}

	return &validate.Result{
		Errors: errs,
	}
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

// MetaValidaton represents a failure of a meta file condition.
type MetaValidaton struct {
	code    int32
	message string
	Name    string
}

// Code returns the code corresponding to the MetaValidation.
func (e *MetaValidaton) Code() int32 {
	return e.code
}

func (e *MetaValidaton) Error() string {
	return e.message
}
