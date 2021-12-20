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
	"fmt"
	"strings"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	mxpkg "github.com/upbound/up/internal/xpkg/dep/marshaler/xpkg"
	"github.com/upbound/up/internal/xpls/validator"
)

// TypeValidator validates the dependency type matches the supplied dependency
// in a meta file.
type TypeValidator struct {
	pkgs map[string]*mxpkg.ParsedPackage
}

// NewTypeValidator returns a new TypeValidator.
func NewTypeValidator(packages map[string]*mxpkg.ParsedPackage) *TypeValidator {
	return &TypeValidator{
		pkgs: packages,
	}
}

// validate validates the dependency versions in a meta file.
func (v *TypeValidator) validate(i int, d v1beta1.Dependency) error {
	got, ok := v.pkgs[d.Package]
	if !ok {
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
