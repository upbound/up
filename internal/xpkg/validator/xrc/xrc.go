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

package xrc

import (
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/upbound/up/internal/xpkg/validator"
)

// XRC defines the XRC validator type.
type XRC struct {
	validators []validator.Validator
}

// New returns a new XRC validator.
func New(schemaValidator validator.Validator) *XRC {
	return &XRC{
		validators: []validator.Validator{schemaValidator},
	}
}

// Validate implements the validator.Validator interface, providing a way to
// validate more than just the strict schema for an XRC.
func (x *XRC) Validate(data interface{}) *validate.Result {
	errs := make([]error, 0)

	for _, v := range x.validators {
		result := v.Validate(data)
		errs = append(errs, result.Errors...)
	}

	return &validate.Result{
		Errors: errs,
	}
}
