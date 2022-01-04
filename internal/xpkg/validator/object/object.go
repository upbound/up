package object

import (
	"encoding/json"
	"errors"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	v1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1beta1ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	xpv1ext "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpv1beta1ext "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/upbound/up/internal/xpkg/validator"
	"github.com/upbound/up/internal/xpkg/validator/crd"
	"github.com/upbound/up/internal/xpkg/validator/xrd"
)

const (
	errObjectNotKnownType = "object is not a known type"
)

// ValidatorsForObj returns a mapping of GVK -> validator for the given runtime.Object.
func ValidatorsForObj(o runtime.Object) (map[schema.GroupVersionKind]validator.Validator, error) {
	validators := make(map[schema.GroupVersionKind]validator.Validator)

	switch rd := o.(type) {
	case *v1beta1ext.CustomResourceDefinition:
		if err := validatorsFromV1Beta1CRD(rd, validators); err != nil {
			return nil, err
		}
	case *v1ext.CustomResourceDefinition:
		if err := validatorsFromV1CRD(rd, validators); err != nil {
			return nil, err
		}
	case *xpv1beta1ext.CompositeResourceDefinition:
		if err := validatorsFromV1Beta1XRD(rd, validators); err != nil {
			return nil, err
		}
	case *xpv1ext.CompositeResourceDefinition:
		if err := validatorsFromV1XRD(rd, validators); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New(errObjectNotKnownType)
	}

	return validators, nil
}

func validatorsFromV1Beta1CRD(c *v1beta1ext.CustomResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {

	internal := &apiextensions.CustomResourceDefinition{}
	if err := v1beta1ext.Convert_v1beta1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(c, internal, nil); err != nil {
		return err
	}

	if internal.Spec.Validation != nil {
		sv, _, err := validation.NewSchemaValidator(internal.Spec.Validation)
		if err != nil {
			return err
		}
		for _, v := range internal.Spec.Versions {
			acc[gvk(internal.Spec.Group, v.Name, internal.Spec.Names.Kind)] = crd.NewV1Beta(sv)
		}
		return nil
	}
	for _, v := range internal.Spec.Versions {
		sv, _, err := validation.NewSchemaValidator(v.Schema)
		if err != nil {
			return err
		}
		acc[gvk(internal.Spec.Group, v.Name, internal.Spec.Names.Kind)] = crd.NewV1Beta(sv)
	}

	return nil
}

func validatorsFromV1CRD(c *v1ext.CustomResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {

	for _, v := range c.Spec.Versions {
		sv, _, err := newV1SchemaValidator(*v.Schema.OpenAPIV3Schema)
		if err != nil {
			return err
		}
		acc[gvk(c.Spec.Group, v.Name, c.Spec.Names.Kind)] = crd.NewV1(sv)
	}

	return nil
}

func validatorsFromV1Beta1XRD(x *xpv1beta1ext.CompositeResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {
	for _, v := range x.Spec.Versions {
		var props v1ext.JSONSchemaProps
		if err := json.Unmarshal(v.Schema.OpenAPIV3Schema.Raw, &props); err != nil {
			return err
		}

		sv, _, err := newV1SchemaValidator(props)
		if err != nil {
			return err
		}

		acc[gvk(x.Spec.Group, v.Name, x.Spec.ClaimNames.Kind)] = xrd.NewV1Beta(sv)
	}
	return nil
}

func validatorsFromV1XRD(x *xpv1ext.CompositeResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {
	for _, ver := range x.Spec.Versions {
		var props v1ext.JSONSchemaProps
		if err := json.Unmarshal(ver.Schema.OpenAPIV3Schema.Raw, &props); err != nil {
			return err
		}

		sv, _, err := newV1SchemaValidator(props)
		if err != nil {
			return err
		}

		acc[gvk(x.Spec.Group, ver.Name, x.Spec.ClaimNames.Kind)] = xrd.NewV1(sv)
	}
	return nil
}

// newSchemaValidator creates an openapi schema validator for the given JSONSchemaProps validation.
func newV1SchemaValidator(schema v1ext.JSONSchemaProps) (*validate.SchemaValidator, *spec.Schema, error) { //nolint:unparam
	// Convert CRD schema to openapi schema
	openapiSchema := &spec.Schema{}
	out := new(apiextensions.JSONSchemaProps)
	if err := v1ext.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&schema, out, nil); err != nil {
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
