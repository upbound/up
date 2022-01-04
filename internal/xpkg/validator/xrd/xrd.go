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
func NewV1(schemaValidator *validate.SchemaValidator) *V1 {
	return &V1{
		validators: []validator.Validator{schemaValidator},
	}
}

// NewV1Beta returns a new V1Beta validator.
func NewV1Beta(schemaValidator *validate.SchemaValidator) *V1Beta {
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
