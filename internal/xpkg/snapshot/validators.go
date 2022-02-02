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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	metav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/xcrd"

	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

const (
	errFmtGetProps        = "cannot get %q properties from validation schema"
	errObjectNotKnownType = "object is not a known type"
	errParseValidation    = "cannot parse validation schema"
)

// ValidatorsForObj returns a mapping of GVK -> validator for the given runtime.Object.
func ValidatorsForObj(o runtime.Object, s *Snapshot) (map[schema.GroupVersionKind]*validator.ObjectValidator, error) { // nolint:gocyclo
	validators := make(map[schema.GroupVersionKind]*validator.ObjectValidator)

	switch rd := o.(type) {
	case *extv1beta1.CustomResourceDefinition:
		if err := validatorsFromV1Beta1CRD(rd, validators); err != nil {
			return nil, err
		}
	case *extv1.CustomResourceDefinition:
		if err := validatorsFromV1CRD(rd, validators); err != nil {
			return nil, err
		}
	case *xpextv1beta1.CompositeResourceDefinition:
		if err := validatorsFromV1Beta1XRD(rd, validators); err != nil {
			return nil, err
		}
	case *xpextv1.CompositeResourceDefinition:
		if err := validatorsFromV1XRD(rd, validators); err != nil {
			return nil, err
		}
		if err := validatorsForV1XRD(rd, validators); err != nil {
			return nil, err
		}
	case *xpextv1.Composition:
		if err := s.validatorsForV1XR(rd, validators); err != nil {
			return nil, err
		}
	case *xpextv1beta1.Composition:
		if err := s.validatorsForV1Beta1XR(rd, validators); err != nil {
			return nil, err
		}
	case *metav1.Configuration:
		if err := s.validatorsForV1Configuration(rd, validators); err != nil {
			return nil, err
		}
	case *metav1alpha1.Configuration:
		if err := s.validatorsForV1Alpha1Configuration(rd, validators); err != nil {
			return nil, err
		}
	case *metav1.Provider:
		if err := s.validatorsForV1Provider(rd, validators); err != nil {
			return nil, err
		}
	case *metav1alpha1.Provider:
		if err := s.validatorsForV1Alpha1Provider(rd, validators); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New(errObjectNotKnownType)
	}

	return validators, nil
}

func validatorsFromV1Beta1CRD(c *extv1beta1.CustomResourceDefinition, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {

	internal := &apiextensions.CustomResourceDefinition{}
	if err := extv1beta1.Convert_v1beta1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(c, internal, nil); err != nil {
		return err
	}

	if internal.Spec.Validation != nil {
		sv, _, err := validation.NewSchemaValidator(internal.Spec.Validation)
		if err != nil {
			return err
		}
		for _, v := range internal.Spec.Versions {
			appendToValidators(gvk(internal.Spec.Group, v.Name, internal.Spec.Names.Kind), acc, sv)
		}
		return nil
	}
	for _, v := range internal.Spec.Versions {
		sv, _, err := validation.NewSchemaValidator(v.Schema)
		if err != nil {
			return err
		}
		appendToValidators(gvk(internal.Spec.Group, v.Name, internal.Spec.Names.Kind), acc, sv)
	}

	return nil
}

func validatorsFromV1CRD(c *extv1.CustomResourceDefinition, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {

	for _, v := range c.Spec.Versions {
		sv, _, err := newV1SchemaValidator(*v.Schema.OpenAPIV3Schema)
		if err != nil {
			return err
		}
		appendToValidators(gvk(c.Spec.Group, v.Name, c.Spec.Names.Kind), acc, sv)
	}

	return nil
}

func validatorsFromV1Beta1XRD(x *xpextv1beta1.CompositeResourceDefinition, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	for _, v := range x.Spec.Versions {
		schema, err := buildSchema(v.Schema.OpenAPIV3Schema)
		if err != nil {
			return err
		}

		sv, _, err := newV1SchemaValidator(*schema)
		if err != nil {
			return err
		}

		if x.Spec.ClaimNames != nil {
			appendToValidators(gvk(x.Spec.Group, v.Name, x.Spec.ClaimNames.Kind), acc, sv)
		}
		appendToValidators(gvk(x.Spec.Group, v.Name, x.Spec.Names.Kind), acc, sv)
	}
	return nil
}

func validatorsFromV1XRD(x *xpextv1.CompositeResourceDefinition, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	errs := validateOpenAPIV3Schema(x)
	if len(errs) != 0 {
		// NOTE (@tnthornton) we're using this as a mechanism to ensure we don't
		// cause upstream validators to panic while evaluating a broken schema.
		// The error contents are meaningless, hence specifically grabbing the
		// first in the slice.
		return errs[0]
	}

	for _, v := range x.Spec.Versions {

		schema, err := buildSchema(v.Schema.OpenAPIV3Schema)
		if err != nil {
			return err
		}

		sv, _, err := newV1SchemaValidator(*schema)
		if err != nil {
			return err
		}

		if x.Spec.ClaimNames != nil {
			appendToValidators(gvk(x.Spec.Group, v.Name, x.Spec.ClaimNames.Kind), acc, sv)
		}
		appendToValidators(gvk(x.Spec.Group, v.Name, x.Spec.Names.Kind), acc, sv)
	}
	return nil
}

func validatorsForV1XRD(x *xpextv1.CompositeResourceDefinition, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	v, err := DefaultXRDValidators()
	if err != nil {
		return err
	}
	appendToValidators(schema.FromAPIVersionAndKind(x.APIVersion, x.Kind), acc, v)
	return nil
}

func (s *Snapshot) validatorsForV1XR(x *xpextv1.Composition, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	v, err := DefaultCompositionValidators(s)
	if err != nil {
		return err
	}
	appendToValidators(x.GroupVersionKind(), acc, v)
	return nil
}

func (s *Snapshot) validatorsForV1Beta1XR(x *xpextv1beta1.Composition, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	v, err := DefaultCompositionValidators(s)
	if err != nil {
		return err
	}
	appendToValidators(x.GroupVersionKind(), acc, v)
	return nil
}

func (s *Snapshot) validatorsForV1Configuration(c *metav1.Configuration, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	v, err := DefaultMetaValidators(s)
	if err != nil {
		return err
	}
	appendToValidators(c.GroupVersionKind(), acc, v)
	return nil
}

func (s *Snapshot) validatorsForV1Alpha1Configuration(c *metav1alpha1.Configuration, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	v, err := DefaultMetaValidators(s)
	if err != nil {
		return err
	}
	appendToValidators(c.GroupVersionKind(), acc, v)
	return nil
}

func (s *Snapshot) validatorsForV1Provider(c *metav1.Provider, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	v, err := DefaultMetaValidators(s)
	if err != nil {
		return err
	}
	appendToValidators(c.GroupVersionKind(), acc, v)
	return nil
}

func (s *Snapshot) validatorsForV1Alpha1Provider(c *metav1alpha1.Provider, acc map[schema.GroupVersionKind]*validator.ObjectValidator) error {
	v, err := DefaultMetaValidators(s)
	if err != nil {
		return err
	}
	appendToValidators(c.GroupVersionKind(), acc, v)
	return nil
}

func appendToValidators(gvk schema.GroupVersionKind, acc map[schema.GroupVersionKind]*validator.ObjectValidator, v validator.Validator) {
	curr, ok := acc[gvk]
	if !ok {
		curr = validator.New(v)
	} else {
		curr.AddToChain(v)
	}
	acc[gvk] = curr
}

func buildSchema(s runtime.RawExtension) (*extv1.JSONSchemaProps, error) {
	schema := xcrd.BaseProps()

	p, required, err := getProps("spec", s)
	if err != nil {
		return nil, errors.Wrapf(err, errFmtGetProps, "spec")
	}
	specProps := schema.Properties["spec"]
	specProps.Required = append(specProps.Required, required...)
	for k, v := range p {
		specProps.Properties[k] = v
	}
	for k, v := range xcrd.CompositeResourceClaimSpecProps() {
		specProps.Properties[k] = v
	}

	schema.Properties["spec"] = specProps

	statusP, statusRequired, err := getProps("status", s)
	if err != nil {
		return nil, errors.Wrapf(err, errFmtGetProps, "status")
	}
	statusProps := schema.Properties["status"]
	statusProps.Required = statusRequired
	for k, v := range statusP {
		statusProps.Properties[k] = v
	}
	for k, v := range xcrd.CompositeResourceStatusProps() {
		statusProps.Properties[k] = v
	}

	schema.Properties["status"] = statusProps

	return schema, nil
}

// newSchemaValidator creates an openapi schema validator for the given JSONSchemaProps validation.
func newV1SchemaValidator(schema extv1.JSONSchemaProps) (*validate.SchemaValidator, *spec.Schema, error) { //nolint:unparam
	// Convert CRD schema to openapi schema
	openapiSchema := &spec.Schema{}
	out := new(apiextensions.JSONSchemaProps)
	if err := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&schema, out, nil); err != nil {
		return nil, nil, err
	}
	if err := validation.ConvertJSONSchemaPropsWithPostProcess(out, openapiSchema, validation.StripUnsupportedFormatsPostProcess); err != nil {
		return nil, nil, err
	}
	return validate.NewSchemaValidator(openapiSchema, nil, "", strfmt.Default), openapiSchema, nil
}

func gvk(group, version, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}
}

func getProps(field string, v runtime.RawExtension) (map[string]extv1.JSONSchemaProps, []string, error) {
	s := &extv1.JSONSchemaProps{}
	if err := json.Unmarshal(v.Raw, s); err != nil {
		return nil, nil, errors.Wrap(err, errParseValidation)
	}

	spec, ok := s.Properties[field]
	if !ok {
		return nil, nil, nil
	}

	return spec.Properties, spec.Required, nil
}
