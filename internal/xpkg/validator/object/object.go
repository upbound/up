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

package object

import (
	"encoding/json"

	"github.com/pkg/errors"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xpextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"

	"github.com/upbound/up/internal/xpkg/validator"
	"github.com/upbound/up/internal/xpkg/validator/crd"
	"github.com/upbound/up/internal/xpkg/validator/xrc"
)

const (
	errFmtGetProps        = "cannot get %q properties from validation schema"
	errObjectNotKnownType = "object is not a known type"
	errParseValidation    = "cannot parse validation schema"
)

// ValidatorsForObj returns a mapping of GVK -> validator for the given runtime.Object.
func ValidatorsForObj(o runtime.Object) (map[schema.GroupVersionKind]validator.Validator, error) {
	validators := make(map[schema.GroupVersionKind]validator.Validator)

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
	default:
		return nil, errors.New(errObjectNotKnownType)
	}

	return validators, nil
}

func validatorsFromV1Beta1CRD(c *extv1beta1.CustomResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {

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

func validatorsFromV1CRD(c *extv1.CustomResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {

	for _, v := range c.Spec.Versions {
		sv, _, err := newV1SchemaValidator(*v.Schema.OpenAPIV3Schema)
		if err != nil {
			return err
		}
		acc[gvk(c.Spec.Group, v.Name, c.Spec.Names.Kind)] = crd.NewV1(sv)
	}

	return nil
}

func validatorsFromV1Beta1XRD(x *xpextv1beta1.CompositeResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {
	for _, v := range x.Spec.Versions {
		schema, err := buildSchema(v.Schema.OpenAPIV3Schema)
		if err != nil {
			return err
		}

		sv, _, err := newV1SchemaValidator(*schema)
		if err != nil {
			return err
		}

		acc[gvk(x.Spec.Group, v.Name, x.Spec.ClaimNames.Kind)] = xrc.New(sv)
	}
	return nil
}

func validatorsFromV1XRD(x *xpextv1.CompositeResourceDefinition, acc map[schema.GroupVersionKind]validator.Validator) error {
	for _, v := range x.Spec.Versions {

		schema, err := buildSchema(v.Schema.OpenAPIV3Schema)
		if err != nil {
			return err
		}

		sv, _, err := newV1SchemaValidator(*schema)
		if err != nil {
			return err
		}

		acc[gvk(x.Spec.Group, v.Name, x.Spec.ClaimNames.Kind)] = xrc.New(sv)
	}
	return nil
}

func buildSchema(s runtime.RawExtension) (*extv1.JSONSchemaProps, error) {
	schema := BaseProps()

	p, required, err := getProps("spec", s)
	if err != nil {
		return nil, errors.Wrapf(err, errFmtGetProps, "spec")
	}
	specProps := schema.Properties["spec"]
	specProps.Required = append(specProps.Required, required...)
	for k, v := range p {
		specProps.Properties[k] = v
	}
	for k, v := range CompositeResourceClaimSpecProps() {
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
	for k, v := range CompositeResourceStatusProps() {
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

// NOTE (@tnthornton) the below functions came from https://github.com/crossplane/crossplane/blob/master/internal/xcrd/schemas.go
// with slight modification to use runtime.RawExtension in the getProps signature.

// BaseProps is a partial OpenAPIV3Schema for the spec fields that Crossplane
// expects to be present for all CRDs that it creates.
func BaseProps() *extv1.JSONSchemaProps {
	return &extv1.JSONSchemaProps{
		Type:     "object",
		Required: []string{"spec"},
		Properties: map[string]extv1.JSONSchemaProps{
			"apiVersion": {
				Type: "string",
			},
			"kind": {
				Type: "string",
			},
			"metadata": {
				// NOTE(muvaf): api-server takes care of validating
				// metadata.
				Type: "object",
			},
			"spec": {
				Type:       "object",
				Properties: map[string]extv1.JSONSchemaProps{},
			},
			"status": {
				Type:       "object",
				Properties: map[string]extv1.JSONSchemaProps{},
			},
		},
	}
}

// CompositeResourceClaimSpecProps is a partial OpenAPIV3Schema for the spec
// fields that Crossplane expects to be present for all published infrastructure
// resources.
func CompositeResourceClaimSpecProps() map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		"compositionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionSelector": {
			Type:     "object",
			Required: []string{"matchLabels"},
			Properties: map[string]extv1.JSONSchemaProps{
				"matchLabels": {
					Type: "object",
					AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
						Allows: true,
						Schema: &extv1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"compositionRevisionRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
		"compositionUpdatePolicy": {
			Type: "string",
			Enum: []extv1.JSON{
				{Raw: []byte(`"Automatic"`)},
				{Raw: []byte(`"Manual"`)},
			},
			Default: &extv1.JSON{Raw: []byte(`"Automatic"`)},
		},
		"resourceRef": {
			Type:     "object",
			Required: []string{"apiVersion", "kind", "name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"apiVersion": {Type: "string"},
				"kind":       {Type: "string"},
				"name":       {Type: "string"},
			},
		},
		"writeConnectionSecretToRef": {
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {Type: "string"},
			},
		},
	}
}

// CompositeResourceStatusProps is a partial OpenAPIV3Schema for the status
// fields that Crossplane expects to be present for all defined or published
// infrastructure resources.
func CompositeResourceStatusProps() map[string]extv1.JSONSchemaProps {
	return map[string]extv1.JSONSchemaProps{
		"conditions": {
			Description: "Conditions of the resource.",
			Type:        "array",
			Items: &extv1.JSONSchemaPropsOrArray{
				Schema: &extv1.JSONSchemaProps{
					Type:     "object",
					Required: []string{"lastTransitionTime", "reason", "status", "type"},
					Properties: map[string]extv1.JSONSchemaProps{
						"lastTransitionTime": {Type: "string", Format: "date-time"},
						"message":            {Type: "string"},
						"reason":             {Type: "string"},
						"status":             {Type: "string"},
						"type":               {Type: "string"},
					},
				},
			},
		},
		"connectionDetails": {
			Type: "object",
			Properties: map[string]extv1.JSONSchemaProps{
				"lastPublishedTime": {Type: "string", Format: "date-time"},
			},
		},
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
