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

package xrd

import (
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/upbound/up/internal/xpkg/validator"
)

// V1 defines the v1.XRD validator type.
type V1 struct {
	validators []validator.Validator
}

// V1Beta defines the v1beta.XRD validator type.
type V1Beta struct {
	validators []validator.Validator
}

// NewV1 returns a new V1 validator.
func NewV1(schemaValidator validator.Validator) *V1 {
	return &V1{
		validators: []validator.Validator{schemaValidator},
	}
}

// NewV1Beta returns a new V1Beta validator.
func NewV1Beta(schemaValidator validator.Validator) *V1Beta {
	return &V1Beta{
		validators: []validator.Validator{schemaValidator},
	}
}

// Validate implements the validator.Validator interface, providing a way to
// validate more than just the strict schema from the XRD.
func (v *V1) Validate(data interface{}) *validate.Result {
	errs := make([]error, 0)

	for _, v := range v.validators {
		result := v.Validate(data)
		errs = append(errs, result.Errors...)
	}

	return &validate.Result{
		Errors: errs,
	}
}

// Validate implements the validator.Validator interface, providing a way to
// validate more than just the strict schema from the XRD.
func (v *V1Beta) Validate(data interface{}) *validate.Result {
	errs := make([]error, 0)

	for _, v := range v.validators {
		result := v.Validate(data)
		errs = append(errs, result.Errors...)
	}

	return &validate.Result{
		Errors: errs,
	}
}
