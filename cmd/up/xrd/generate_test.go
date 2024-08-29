// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xrd

import (
	"encoding/json"
	"testing"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestInferProperty tests the inferProperty function.
func TestInferProperty(t *testing.T) {
	type want struct {
		output extv1.JSONSchemaProps
	}

	cases := map[string]struct {
		input interface{}
		want  want
	}{
		"StringType": {
			input: "hello",
			want: want{
				output: extv1.JSONSchemaProps{Type: "string"},
			},
		},
		"IntegerType": {
			input: 42,
			want: want{
				output: extv1.JSONSchemaProps{Type: "integer"},
			},
		},
		"FloatType": {
			input: 3.14,
			want: want{
				output: extv1.JSONSchemaProps{Type: "number"},
			},
		},
		"BooleanType": {
			input: true,
			want: want{
				output: extv1.JSONSchemaProps{Type: "boolean"},
			},
		},
		"ObjectType": {
			input: map[string]interface{}{
				"key": "value",
			},
			want: want{
				output: extv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"key": {Type: "string"},
					},
				},
			},
		},
		"ArrayTypeWithElements": {
			input: []interface{}{"one", "two"},
			want: want{
				output: extv1.JSONSchemaProps{
					Type: "array",
					Items: &extv1.JSONSchemaPropsOrArray{
						Schema: &extv1.JSONSchemaProps{Type: "string"},
					},
				},
			},
		},
		"ArrayTypeEmpty": {
			input: []interface{}{},
			want: want{
				output: extv1.JSONSchemaProps{
					Type: "array",
					Items: &extv1.JSONSchemaPropsOrArray{
						Schema: &extv1.JSONSchemaProps{Type: "object"},
					},
				},
			},
		},
		"UnknownType": {
			input: nil,
			want: want{
				output: extv1.JSONSchemaProps{Type: "string"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := inferProperty(tc.input)

			if diff := cmp.Diff(got, tc.want.output); diff != "" {
				t.Errorf("inferProperty() -got, +want:\n%s", diff)
			}
		})
	}
}

// TestInferProperties tests the inferProperties function.
func TestInferProperties(t *testing.T) {
	type want struct {
		output map[string]extv1.JSONSchemaProps
	}

	cases := map[string]struct {
		input map[string]interface{}
		want  want
	}{
		"SimpleObject": {
			input: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			want: want{
				output: map[string]extv1.JSONSchemaProps{
					"key1": {Type: "string"},
					"key2": {Type: "integer"},
				},
			},
		},
		"NestedObject": {
			input: map[string]interface{}{
				"nested": map[string]interface{}{
					"key": true,
				},
			},
			want: want{
				output: map[string]extv1.JSONSchemaProps{
					"nested": {
						Type: "object",
						Properties: map[string]extv1.JSONSchemaProps{
							"key": {Type: "boolean"},
						},
					},
				},
			},
		},
		"ArrayInObject": {
			input: map[string]interface{}{
				"array": []interface{}{"a", "b"},
			},
			want: want{
				output: map[string]extv1.JSONSchemaProps{
					"array": {
						Type: "array",
						Items: &extv1.JSONSchemaPropsOrArray{
							Schema: &extv1.JSONSchemaProps{Type: "string"},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := inferProperties(tc.input)

			if diff := cmp.Diff(got, tc.want.output); diff != "" {
				t.Errorf("inferProperties() -got, +want:\n%s", diff)
			}
		})
	}
}

// TestNewXRD tests the newXRD function.
func TestNewXRD(t *testing.T) {
	type want struct {
		xrd *v1.CompositeResourceDefinition
		err error
	}

	cases := map[string]struct {
		inputYAML    string
		customPlural string
		want         want
	}{
		"XRXEKS": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: XEKS
metadata:
  name: test
spec:
  parameters:
    id: test
    region: eu-central-1
`,
			customPlural: "xeks",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "XEKS is the Schema for the XEKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "XEKSSpec defines the desired state of XEKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "XEKSStatus defines the observed state of XEKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"XRCEKS": {
			inputYAML: `
apiVersion: aws.u5d.io/v1
kind: EKS
metadata:
  name: test
  namespace: test-namespace
spec:
  parameters:
    id: test
    region: eu-central-1
`,
			customPlural: "eks",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "xeks.aws.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "aws.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "XEKS",
							Plural:     "xeks",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "EKS is the Schema for the EKS API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "EKSSpec defines the desired state of EKS.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"id": {
																Type: "string",
															},
															"region": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "EKSStatus defines the observed state of EKS.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
						ClaimNames: &extv1.CustomResourceDefinitionNames{
							Kind:   "EKS",
							Plural: "eks",
						},
					},
				},
				err: nil,
			},
		},
		"XRPostgres": {
			inputYAML: `
apiVersion: database.u5d.io/v1
kind: Postgres
metadata:
  name: test
spec:
  parameters:
    version: "13"
`,
			customPlural: "Postgreses",
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "postgreses.database.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "database.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Postgres",
							Plural:     "postgreses",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Postgres is the Schema for the Postgres API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "PostgresSpec defines the desired state of Postgres.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"version": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "PostgresStatus defines the observed state of Postgres.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"XRBucket": {
			inputYAML: `
apiVersion: storage.u5d.io/v1
kind: Bucket
metadata:
  name: test
spec:
  parameters:
    storage: "13"
`,
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "buckets.storage.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "storage.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Bucket",
							Plural:     "buckets",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Bucket is the Schema for the Bucket API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "BucketSpec defines the desired state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"storage": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "BucketStatus defines the observed state of Bucket.",
												Type:        "object",
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
		"XRBucketWithStatus": {
			inputYAML: `
apiVersion: storage.u5d.io/v1
kind: Bucket
metadata:
  name: test
spec:
  parameters:
    storage: "13"
status:
  bucketName: test
`,
			want: want{
				xrd: &v1.CompositeResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1",
						Kind:       "CompositeResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "buckets.storage.u5d.io",
					},
					Spec: v1.CompositeResourceDefinitionSpec{
						Group: "storage.u5d.io",
						Names: extv1.CustomResourceDefinitionNames{
							Categories: []string{"crossplane"},
							Kind:       "Bucket",
							Plural:     "buckets",
						},
						Versions: []v1.CompositeResourceDefinitionVersion{
							{
								Name:          "v1",
								Referenceable: true,
								Served:        true,
								Schema: &v1.CompositeResourceValidation{
									OpenAPIV3Schema: jsonSchemaPropsToRawExtension(&extv1.JSONSchemaProps{
										Description: "Bucket is the Schema for the Bucket API.",
										Type:        "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Description: "BucketSpec defines the desired state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"parameters": {
														Type: "object",
														Properties: map[string]extv1.JSONSchemaProps{
															"storage": {
																Type: "string",
															},
														},
													},
												},
											},
											"status": {
												Description: "BucketStatus defines the observed state of Bucket.",
												Type:        "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"bucketName": {
														Type: "string",
													},
												},
											},
										},
										Required: []string{"spec"},
									}),
								},
							},
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := newXRD([]byte(tc.inputYAML), tc.customPlural)

			if diff := cmp.Diff(got, tc.want.xrd, cmpopts.IgnoreFields(extv1.JSONSchemaProps{}, "Required")); diff != "" {
				t.Errorf("newXRD() -got, +want:\n%s", diff)
			}

			if diff := cmp.Diff(err, tc.want.err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("newXRD() error -got, +want:\n%s", diff)
			}
		})
	}
}

// helper function to convert JSONSchemaProps to RawExtension
func jsonSchemaPropsToRawExtension(schema *extv1.JSONSchemaProps) runtime.RawExtension {
	schemaBytes, _ := json.Marshal(schema)
	return runtime.RawExtension{Raw: schemaBytes}
}
