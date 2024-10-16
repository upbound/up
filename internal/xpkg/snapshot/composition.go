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
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	verrors "k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	icomposite "github.com/crossplane/crossplane/controller/apiextensions/composite"
	icompositions "github.com/crossplane/crossplane/controller/apiextensions/compositions"

	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

const (
	resources = "spec.resources"

	errFmt                  = "%s (%s)"
	errInvalidValidationFmt = "invalid validation result returned for %s"
	resourceBaseFmt         = "spec.resources[%d].base.%s"

	errIncorrectErrType = "incorrect validaton error type seen"
	errInvalidType      = "invalid type passed in, expected Unstructured"
)

// CompositionValidator defines a validator for compositions.
type CompositionValidator struct {
	s          *Snapshot
	validators []compositionValidator
}

// DefaultCompositionValidators returns a new Composition validator.
func DefaultCompositionValidators(s *Snapshot) (validator.Validator, error) {
	return &CompositionValidator{
		s: s,
		validators: []compositionValidator{
			NewPatchesValidator(s),
		},
	}, nil
}

// Validate performs validation rules on the given data input per the rules
// defined for the Validator.
func (c *CompositionValidator) Validate(ctx context.Context, data any) *validate.Result {
	errs := []error{}

	comp, err := c.marshal(data)
	if err != nil {
		return validator.Nop
	}

	compRefGVK := schema.FromAPIVersionAndKind(
		comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind,
	)

	r := icomposite.NewReconciler(resource.CompositeKind(compRefGVK), icomposite.WithLogger(c.s.log))
	cds, err := r.Reconcile(ctx, comp)
	if err != nil {
		// some validation errors occur during reconciliation that we want to
		// send to the end user.
		ie := &validator.Validation{
			TypeCode: validator.ErrorTypeCode,
			Message:  err.Error(),
			Name:     resources,
		}
		errs = append(errs, ie)
	}

	if len(errs) == 0 {
		for i, cd := range cds {
			for _, v := range c.validators {
				errs = append(errs, v.validate(ctx, i, cd.Resource)...)
			}
		}
	}

	return &validate.Result{
		Errors: errs,
	}
}

func (c *CompositionValidator) marshal(data any) (*xpextv1.Composition, error) {
	u, ok := data.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New(errInvalidType)
	}

	b, err := u.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var mcomp xpextv1.Composition
	err = json.Unmarshal(b, &mcomp)
	if err != nil {
		return nil, err
	}

	// convert v1.Composition to v1alpha1.CompositionRevision back to
	// v1.Composition to take advantage of default fields being set for various
	// sub objects within the v1.Composition definition.
	crev := icompositions.NewCompositionRevision(&mcomp, 1)
	comp := icomposite.AsComposition(crev)

	return comp, nil
}

type compositionValidator interface {
	validate(context.Context, int, resource.Composed) []error
}

// PatchesValidator validates the patches fields of a Composition.
type PatchesValidator struct {
	s *Snapshot
}

// NewPatchesValidator returns a new PatchesValidator.
func NewPatchesValidator(s *Snapshot) *PatchesValidator {
	return &PatchesValidator{
		s: s,
	}
}

// Validate validates that the composed resource is valid per the base
// resource's schema.
func (p *PatchesValidator) validate(ctx context.Context, idx int, cd resource.Composed) []error {
	cdgvk := cd.GetObjectKind().GroupVersionKind()
	v, ok := p.s.validators[cdgvk]
	if !ok {
		return gvkDNEWarning(cdgvk, fmt.Sprintf(resourceBaseFmt, idx, "apiVersion"))
	}

	result := v.Validate(ctx, cd)
	if result != nil {
		errs := []error{}
		for _, e := range result.Errors {
			var ve *verrors.Validation
			if !errors.As(e, &ve) {
				return []error{errors.New(errIncorrectErrType)}
			}
			ie := &validator.Validation{
				TypeCode: ve.Code(),
				Message:  fmt.Sprintf(errFmt, ve.Error(), cdgvk),
				Name:     fmt.Sprintf(resourceBaseFmt, idx, ve.Name),
			}
			errs = append(errs, ie)
		}
		return errs
	}

	return []error{fmt.Errorf(errInvalidValidationFmt, cdgvk)}
}
