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
	"k8s.io/kube-openapi/pkg/validation/validate"
)

// All code responses can be used to differentiate errors for different handling
// by the consumer
const (
	// WarningTypeCode indicates a warning is being returned.
	WarningTypeCode = 100
	// ErrorTypeCode indicates an error is being returned.
	ErrorTypeCode = 500

	// NOTE(@tnthornton) api-server uses error code 422 and 600+ to indicate
	// validation errors. As long as we're deferring to their logic for
	// k8s type validations, we need to be sure to not overload those error
	// codes.
)

// Nop is used for no-op validator results.
var Nop = &validate.Result{}

// A Validator validates data and returns a validation result.
type Validator interface {
	Validate(data interface{}) *validate.Result
}

// Validation represents a failure of a file condition.
type Validation struct {
	TypeCode int32
	Message  string
	Name     string
}

// Code returns the code corresponding to the MetaValidation.
func (e *Validation) Code() int32 {
	return e.TypeCode
}

func (e *Validation) Error() string {
	return e.Message
}

// ObjectValidator provides a mechanism for grouping various validators for a
// given object type.
type ObjectValidator struct {
	chain []Validator
}

// New returns a new ObjectValidator.
func New(validators ...Validator) *ObjectValidator {
	chain := make([]Validator, 0)

	return &ObjectValidator{
		chain: append(chain, validators...),
	}
}

// Validate implements the validator.Validator interface, providing a way to
// validate more than just the strict schema for a given runtime.Object.
func (o *ObjectValidator) Validate(data interface{}) *validate.Result {
	errs := []error{}

	for _, v := range o.chain {
		result := v.Validate(data)
		errs = append(errs, result.Errors...)
	}

	return &validate.Result{
		Errors: errs,
	}
}

// AddToChain adds the given validators to the internal validation chain for
// the ObjectValidator.
func (o *ObjectValidator) AddToChain(validators ...Validator) {
	o.chain = append(o.chain, validators...)
}
