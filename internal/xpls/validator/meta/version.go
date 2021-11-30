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
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpls/validator"
)

const (
	versionField = "version"

	errPackageDNEFmt   = "Package %s does not exist locally. Please run `up xpkg dep` to fix."
	errVersionDENFmt   = "Version matching %s does not exist locally. Please run `up xpkg dep` to fix."
	errWrongPkgTypeFmt = "Incorrect package type. '%s' does not match type for %s of '%s'"
)

// VersionValidator is used to validate the dependency versions in a meta file.
type VersionValidator struct {
	m *Meta
}

// NewVersionValidator returns a new VersionValidator.
func NewVersionValidator(m *Meta) *VersionValidator {
	return &VersionValidator{
		m: m,
	}
}

// Validate validates the dependency versions in a meta file.
func (v *VersionValidator) Validate(data interface{}) *validate.Result {
	pkg, err := v.m.Marshal(data)
	if err != nil {
		// TODO(@tnthornton) add debug logging
		return validator.Nop
	}

	errs := make([]error, 0)

	for i, d := range pkg.GetDependencies() {
		cd := manager.ConvertToV1beta1(d)
		errs = append(errs, v.validateVersion(i, cd))
	}

	return &validate.Result{
		Errors: errs,
	}
}

func (v *VersionValidator) validateVersion(i int, d v1beta1.Dependency) error {
	// check explicit version
	// TODO(@tnthornton) move this into the Snapshot. Versions shouldn't change
	// within the lifespan of a snapshot.
	vers, err := v.m.manager.Versions(context.Background(), d)
	if err != nil {
		// TODO(@tnthornton) add debug logging here
		return nil
	}
	if len(vers) == 0 {
		return &validator.MetaValidaton{
			Name:    fmt.Sprintf(dependsOnPathFmt, i, strings.ToLower(string(d.Type))),
			Message: fmt.Sprintf(errPackageDNEFmt, d.Package),
		}
	}
	if !versionMatch(d.Constraints, vers) {
		return &validator.MetaValidaton{
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
