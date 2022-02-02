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
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kube-openapi/pkg/validation/validate"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/xcrd"

	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

var (
	mapKeyRE = regexp.MustCompile(`(\[([a-zA-Z]*)\])`)
)

// XRDValidator defines a validator for xrd files.
type XRDValidator struct {
	validators []xrdValidator
}

// DefaultXRDValidators returns a new Meta validator.
func DefaultXRDValidators() (validator.Validator, error) {
	validators := []xrdValidator{
		NewXRDSchemaValidator(),
	}

	return &XRDValidator{
		validators: validators,
	}, nil
}

// Validate performs validation rules on the given data input per the rules
// defined for the Validator.
func (x *XRDValidator) Validate(data interface{}) *validate.Result {
	xrd, err := x.Marshal(data)
	if err != nil {
		// TODO(@tnthornton) add debug logging
		return validator.Nop
	}

	errs := make([]error, 0)

	for _, v := range x.validators {
		errs = append(errs, v.validate(xrd)...)
	}

	return &validate.Result{
		Errors: errs,
	}
}

// Marshal marshals the given data object into a Pkg, errors otherwise.
func (x *XRDValidator) Marshal(data interface{}) (*xpextv1.CompositeResourceDefinition, error) {
	u, ok := data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New("invalid type passed in, expected Unstructured")
	}

	b, err := u.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var xrd xpextv1.CompositeResourceDefinition
	err = json.Unmarshal(b, &xrd)
	if err != nil {
		return nil, err
	}

	return &xrd, nil
}

type xrdValidator interface {
	validate(*xpextv1.CompositeResourceDefinition) []error
}

// XRDSchemaValidator validates XRD schema definitions.
type XRDSchemaValidator struct{}

// NewXRDSchemaValidator returns a new XRDSchemaValidator.
func NewXRDSchemaValidator() *XRDSchemaValidator {
	return &XRDSchemaValidator{}
}

func (v *XRDSchemaValidator) validate(xrd *xpextv1.CompositeResourceDefinition) []error {

	errs := validateOpenAPIV3Schema(xrd)

	errList := []error{}

	for _, e := range errs {
		var fe *field.Error
		if errors.As(e, &fe) {
			fieldValue := fe.Field

			path := cleanFieldPath(fieldValue)
			errList = append(errList, &validator.Validation{
				TypeCode: validator.ErrorTypeCode,
				Name:     path,
				Message:  fmt.Sprintf("%s %s", path, fe.ErrorBody()),
			},
			)
		}
	}

	return errList
}

// validateOpenAPIV3Schema validates the spec.versions[*].schema.openAPIV3Schema
// section of the given XRD definition.
func validateOpenAPIV3Schema(xrd *xpextv1.CompositeResourceDefinition) []error {
	crd, err := xcrd.ForCompositeResource(xrd)
	if err != nil {
		return nil
	}

	extv1.SetObjectDefaults_CustomResourceDefinition(crd)

	internal := &apiextensions.CustomResourceDefinition{}
	if err := extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil); err != nil {
		return nil
	}
	felist := validation.ValidateCustomResourceDefinition(internal)
	if felist != nil {
		return felist.ToAggregate().Errors()
	}
	return nil
}

func cleanFieldPath(fieldVal string) string {
	fns := []cleaner{
		replaceValidation,
		replaceMapKeys,
		trimInvalidDollarSign,
	}

	cleaned := fieldVal
	for _, f := range fns {
		cleaned = f(cleaned)
	}

	return cleaned
}

type cleaner func(string) string

// if the validations were all moved to spec.validation, update the path
// to point to spec.version[0]
func replaceValidation(fieldVal string) string {
	return strings.Replace(fieldVal, "spec.validation", "spec.versions[0].schema", 1)
}

// paths are returned from CRD validations using map[key].field notation
func replaceMapKeys(fieldVal string) string {
	return mapKeyRE.ReplaceAllString(fieldVal, ".$2")
}

func trimInvalidDollarSign(fieldVal string) string {
	if idx := strings.Index(fieldVal, ".$"); idx != -1 {
		return fieldVal[:idx]
	}
	return fieldVal
}
