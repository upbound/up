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
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	xpextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestCleanFieldPath(t *testing.T) {
	type args struct {
		path string
	}
	type want struct {
		cleaned string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ShouldExpandMapKeysIntoYamlPathNotation": {
			reason: "map[key] syntax should be replaced by valid yaml path",
			args: args{
				path: "spec.versions[0].schema.openAPIV3Schema.properties[spec].properties[parameters].properties[storageGB]",
			},
			want: want{
				cleaned: "spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.parameters.properties.storageGB",
			},
		},
		"ShouldRemovePathSegmentsWith$Ref": {
			reason: "An Embedded .$ref within the yaml path is invalid and should be removed.",
			args: args{
				path: "spec.versions[0].schema.openAPIV3Schema.$ref",
			},
			want: want{
				cleaned: "spec.versions[0].schema.openAPIV3Schema",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got := cleanFieldPath(tc.args.path)

			if diff := cmp.Diff(tc.want.cleaned, got); diff != "" {
				t.Errorf("\n%s\nCleanFieldPath(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestValidateOpenAPIV3Schema(t *testing.T) {
	type args struct {
		xrd *xpextv1.CompositeResourceDefinition
	}
	type want struct {
		errs []error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrDueToRootTypeNotSpecified": {
			reason: "type at the root is a required property for the schema definition.",
			args: args{
				xrd: &xpextv1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xpostgresqlinstances.database.example.org",
					},
					Spec: xpextv1.CompositeResourceDefinitionSpec{
						Group: "database.example.org",
						Names: v1.CustomResourceDefinitionNames{
							Kind:   "XPostgreSQLInstance",
							Plural: "xpostgresqlinstances",
						},
						ClaimNames: &v1.CustomResourceDefinitionNames{
							Kind:   "PostgreSQLInstance",
							Plural: "postgresqlinstances",
						},
						Versions: []xpextv1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1alpha1",
								Served:        true,
								Referenceable: true,
								Schema: &xpextv1.CompositeResourceValidation{
									OpenAPIV3Schema: runtime.RawExtension{
										Raw: []byte(`{
											"properties": {
												"spec": {
													"properties": {
														"parameters": {
															"properties": {
																"storageGB": {
																	"type":"integer"
																}
															},
															"required":[
																"storageGB"
															],
															"type":"object"
														}
													},
													"required": [
														"parameters"
													],
													"type": "object"
												}
											}
										}`),
									},
								},
							},
						},
					},
				},
			},
			want: want{
				errs: []error{
					&field.Error{
						Type:     "FieldValueRequired",
						Field:    "spec.validation.openAPIV3Schema.type",
						BadValue: string(""),
						Detail:   "must not be empty at the root",
					},
				},
			},
		},
		"ErrDueTo$RefIncluded": {
			reason: "Defining $ref within the schema is invalid.",
			args: args{
				xrd: &xpextv1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xpostgresqlinstances.database.example.org",
					},
					Spec: xpextv1.CompositeResourceDefinitionSpec{
						Group: "database.example.org",
						Names: v1.CustomResourceDefinitionNames{
							Kind:   "XPostgreSQLInstance",
							Plural: "xpostgresqlinstances",
						},
						ClaimNames: &v1.CustomResourceDefinitionNames{
							Kind:   "PostgreSQLInstance",
							Plural: "postgresqlinstances",
						},
						Versions: []xpextv1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1alpha1",
								Served:        true,
								Referenceable: true,
								Schema: &xpextv1.CompositeResourceValidation{
									OpenAPIV3Schema: runtime.RawExtension{
										Raw: []byte(`{
											"properties": {
												"spec": {
													"properties": {
														"parameters": {
															"properties": {
																"storageGB": {
																	"$ref":"blah"
																}
															},
															"required":[
																"storageGB"
															],
															"type":"object"
														}
													},
													"required": [
														"parameters"
													],
													"type": "object"
												}
											},
											"type": "object"
										}`),
									},
								},
							},
						},
					},
				},
			},
			want: want{
				errs: []error{
					&field.Error{
						Type:     "FieldValueForbidden",
						Field:    "spec.validation.openAPIV3Schema.properties[spec].properties[parameters].properties[storageGB].$ref",
						BadValue: string(""),
						Detail:   "$ref is not supported",
					},
				},
			},
		},
		"NoSchemaDefined": {
			reason: "Not defining a schema is valid.",
			args: args{
				xrd: &xpextv1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xpostgresqlinstances.database.example.org",
					},
					Spec: xpextv1.CompositeResourceDefinitionSpec{
						Group: "database.example.org",
						Names: v1.CustomResourceDefinitionNames{
							Kind:   "XPostgreSQLInstance",
							Plural: "xpostgresqlinstances",
						},
						ClaimNames: &v1.CustomResourceDefinitionNames{
							Kind:   "PostgreSQLInstance",
							Plural: "postgresqlinstances",
						},
						Versions: []xpextv1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1alpha1",
								Served:        true,
								Referenceable: true,
							},
						},
					},
				},
			},
			want: want{
				errs: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got := validateOpenAPIV3Schema(tc.args.xrd)

			if diff := cmp.Diff(tc.want.errs, got); diff != "" {
				t.Errorf("\n%s\nValidateOpenAPIV3Schema(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
