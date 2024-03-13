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
	"testing"

	"github.com/google/go-cmp/cmp"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/validate"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/up/internal/xpkg/scheme"
	"github.com/upbound/up/internal/xpkg/snapshot/validator"
)

func TestCompositionValidation(t *testing.T) {
	objScheme, _ := scheme.BuildObjectScheme()
	metaScheme, _ := scheme.BuildMetaScheme()
	ctx := context.Background()

	s := &Snapshot{
		objScheme:  objScheme,
		metaScheme: metaScheme,
		log:        logging.NewNopLogger(),
	}

	type args struct {
		data       runtime.Object
		validators map[schema.GroupVersionKind]validator.Validator
	}
	type want struct {
		result *validate.Result
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ComposedResourceMissingValidator": {
			reason: "Base resource GVK is missing a validator, we expect to get a warning indicating that.",
			args: args{
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Resources: []v1.ComposedTemplate{
							{
								Base: runtime.RawExtension{Raw: []byte(`{"apiVersion": "database.aws.crossplane.io/v1beta1", "kind":"RDSInstance"}`)},
							},
						},
					},
				},
				validators: make(map[schema.GroupVersionKind]validator.Validator), // empty validators map
			},
			want: want{
				&validate.Result{
					Errors: []error{
						&validator.Validation{
							TypeCode: validator.WarningTypeCode,
							Message:  "no definition found for resource (database.aws.crossplane.io/v1beta1, Kind=RDSInstance)",
							Name:     "spec.resources[0].base.apiVersion",
						},
					},
				},
			},
		},
		"ComposedResourceMissingRequiredField": {
			reason: "Base resource definition is missing a required field, we expect to get an error for that.",
			args: args{
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Resources: []v1.ComposedTemplate{
							{
								Base: runtime.RawExtension{Raw: []byte(`{
									"apiVersion": "acm.aws.crossplane.io/v1alpha1",
									"kind":"Certificate",
									"spec": {
										"forProvider": {
											"domainName": "dn",
											"region": "us-west-2",
											"tags": [
												{"key": "k", "value": "v"}
											]
										},
										"writeConnectionSecretToRef": {
											"namespace": "default"
										}
									}
								}`)},
							},
						},
					},
				},
				validators: func() map[schema.GroupVersionKind]validator.Validator {
					v, _ := s.validatorsFromBytes(ctx, testSingleVersionCRD)
					return v
				}(),
			},
			want: want{
				&validate.Result{
					Errors: []error{
						&validator.Validation{
							TypeCode: 602,
							Message:  "spec.writeConnectionSecretToRef.name in body is required (acm.aws.crossplane.io/v1alpha1, Kind=Certificate)",
							Name:     "spec.resources[0].base.spec.writeConnectionSecretToRef.name",
						},
					},
				},
			},
		},
		"ComposedResourceRequiredFieldProvidedByPatch": {
			reason: "Base resource definition has a required field patched via patches.",
			args: args{
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Resources: []v1.ComposedTemplate{
							{
								Base: runtime.RawExtension{Raw: []byte(`{
									"apiVersion": "acm.aws.crossplane.io/v1alpha1",
									"kind":"Certificate",
									"spec": {
										"forProvider": {
											"domainName": "dn",
											"region": "us-west-2",
											"tags": [
												{"key": "k", "value": "v"}
											]
										},
										"writeConnectionSecretToRef": {
											"namespace": "default"
										}
									}
								}`)},
								Patches: []v1.Patch{
									{
										FromFieldPath: ptr.To("metadata.uid"),
										ToFieldPath:   ptr.To("spec.writeConnectionSecretToRef.name"),
										Transforms: []v1.Transform{
											{
												Type: "string",
												String: &v1.StringTransform{
													Format: ptr.To("%s-postgresql"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
				validators: func() map[schema.GroupVersionKind]validator.Validator {
					v, _ := s.validatorsFromBytes(ctx, testSingleVersionCRD)
					return v
				}(),
			},
			want: want{
				result: &validate.Result{Errors: []error{}},
			},
		},
		"ComposedResourceRequiredFieldProvidedByPatchSet": {
			reason: "Base resource definition has a required field patched via patchSet.",
			args: args{
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						PatchSets: []v1.PatchSet{
							{
								Name: "connectionSecretRef",
								Patches: []v1.Patch{
									{
										FromFieldPath: ptr.To("metadata.uid"),
										ToFieldPath:   ptr.To("spec.writeConnectionSecretToRef.name"),
										Transforms: []v1.Transform{
											{
												Type: "string",
												String: &v1.StringTransform{
													Format: ptr.To("%s-postgresql"),
												},
											},
										},
									},
								},
							},
						},
						Resources: []v1.ComposedTemplate{
							{
								Base: runtime.RawExtension{Raw: []byte(`{
									"apiVersion": "acm.aws.crossplane.io/v1alpha1",
									"kind":"Certificate",
									"spec": {
										"forProvider": {
											"domainName": "dn",
											"region": "us-west-2",
											"tags": [
												{"key": "k", "value": "v"}
											]
										},
										"writeConnectionSecretToRef": {
											"namespace": "default"
										}
									}
								}`)},
								Patches: []v1.Patch{
									{
										Type:         v1.PatchTypePatchSet,
										PatchSetName: ptr.To("connectionSecretRef"),
									},
								},
							},
						},
					},
				},
				validators: func() map[schema.GroupVersionKind]validator.Validator {
					v, _ := s.validatorsFromBytes(ctx, testSingleVersionCRD)
					return v
				}(),
			},
			want: want{
				result: &validate.Result{Errors: []error{}},
			},
		},
		"ComposedResourceRequiredFieldProvidedByPatchThroughWriteConnectionSecretToRef": {
			reason: "Base resource definition has its writeConnectionSecretToRef namespace and name coming from the XR/C",
			args: args{
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Resources: []v1.ComposedTemplate{
							{
								Base: runtime.RawExtension{Raw: []byte(`{
									"apiVersion": "acm.aws.crossplane.io/v1alpha1",
									"kind":"Certificate",
									"spec": {
										"forProvider": {
											"domainName": "dn",
											"region": "us-west-2",
											"tags": [
												{"key": "k", "value": "v"}
											]
										},
										"writeConnectionSecretToRef": {
												"namespace": "default"
										}
									}
								}`)},
								Patches: []v1.Patch{
									{
										FromFieldPath: ptr.To("spec.writeConnectionSecretToRef.name"),
										ToFieldPath:   ptr.To("spec.writeConnectionSecretToRef.name"),
									},
								},
							},
						},
					},
				},
				validators: func() map[schema.GroupVersionKind]validator.Validator {
					v, _ := s.validatorsFromBytes(ctx, testSingleVersionCRD)
					return v
				}(),
			},
			want: want{
				&validate.Result{
					Errors: []error{},
				},
			},
		},
		"ComposedResourceHasMixedNamingResources": {
			reason: "Base resources must either be all named or all not named.",
			args: args{
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Resources: []v1.ComposedTemplate{
							{
								Name: ptr.To("r1"),
							},
							{},
						},
					},
				},
				validators: func() map[schema.GroupVersionKind]validator.Validator {
					v, _ := s.validatorsFromBytes(ctx, testSingleVersionCRD)
					return v
				}(),
			},
			want: want{
				&validate.Result{
					Errors: []error{
						&validator.Validation{
							TypeCode: validator.ErrorTypeCode,
							Message:  "spec.resources[1].name: Required value: cannot mix named and anonymous resources, all resources must have a name or none must have a name",
							Name:     "spec.resources",
						},
					},
				},
			},
		},
		"ComposedResourceHasDuplicateNamedResources": {
			reason: "Base resources must be uniquely named.",
			args: args{
				data: &v1.Composition{
					TypeMeta: apimetav1.TypeMeta{
						Kind:       v1.CompositionKind,
						APIVersion: v1.SchemeGroupVersion.String(),
					},
					Spec: v1.CompositionSpec{
						Resources: []v1.ComposedTemplate{
							{
								Name: ptr.To("r1"),
							},
							{
								Name: ptr.To("r1"),
							},
						},
					},
				},
				validators: func() map[schema.GroupVersionKind]validator.Validator {
					v, _ := s.validatorsFromBytes(ctx, testSingleVersionCRD)
					return v
				}(),
			},
			want: want{
				&validate.Result{
					Errors: []error{
						&validator.Validation{
							TypeCode: validator.ErrorTypeCode,
							Message:  `spec.resources[1].name: Duplicate value: "r1"`,
							Name:     "spec.resources",
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s.validators = tc.args.validators

			// convert runtime.Object -> *unstructured.Unstructured
			b, err := json.Marshal(tc.args.data)
			// we shouldn't see an error from Marshaling
			if diff := cmp.Diff(err, nil, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCompositionValidation(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			var u unstructured.Unstructured
			json.Unmarshal(b, &u)

			v, _ := DefaultCompositionValidators(s)

			result := v.Validate(ctx, &u)

			if diff := cmp.Diff(tc.want.result, result); diff != "" {
				t.Errorf("\n%s\nCompositionValidation(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
