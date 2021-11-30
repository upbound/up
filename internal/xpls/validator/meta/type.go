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

	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"

	"github.com/upbound/up/internal/xpkg/dep/manager"
	"github.com/upbound/up/internal/xpls/validator"
)

// TypeValidator validates the dependency type matches the supplied dependency
// in a meta file.
type TypeValidator struct {
	m *Meta
}

// NewTypeValidator returns a new TypeValidator.
func NewTypeValidator(m *Meta) *TypeValidator {
	return &TypeValidator{
		m: m,
	}
}

// Validate validates the dependency versions in a meta file.
func (v *TypeValidator) Validate(data interface{}) *validate.Result {
	pkg, err := v.m.Marshal(data)
	if err != nil {
		// TODO(@tnthornton) add debug logging
		return validator.Nop
	}

	errs := make([]error, 0)

	for i, d := range pkg.GetDependencies() {
		cd := manager.ConvertToV1beta1(d)
		errs = append(errs, v.validateType(i, cd))
	}

	return &validate.Result{
		Errors: errs,
	}
}

func (v *TypeValidator) validateType(i int, d v1beta1.Dependency) error {
	got, ok := v.m.packages[d.Package]
	if !ok {
		return nil
	}
	if got.Type() != d.Type {
		return &validator.MetaValidaton{
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
