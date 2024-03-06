// Copyright 2023 Upbound Inc.
// All rights reserved

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSpaceQueryValidateCreate(t *testing.T) {
	tests := map[string]struct {
		reason   string
		query    SpaceQuery
		wantErrs []string
	}{
		"Namespace": {
			reason: "SpaceQuery is cluster scoped and must not have a namespace",
			query: SpaceQuery{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec: &QuerySpec{},
			},
			wantErrs: []string{
				`metadata.namespace: Forbidden: not allowed on this type`,
			},
		},
		"NoName": {
			reason: "SpaceQuery name is defaulted",
			query: SpaceQuery{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       &QuerySpec{},
			},
		},
		"Name": {
			reason: "SpaceQuery name does not matter",
			query: SpaceQuery{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						Filter: QueryTopLevelFilter{
							ControlPlane: QueryFilterControlPlane{
								Namespace: "foo",
								Name:      "foo",
							},
						},
					},
				},
			},
		},
		"NoSpec": {
			reason: "SpaceQuery must have a spec",
			query: SpaceQuery{
				ObjectMeta: metav1.ObjectMeta{},
			},
			wantErrs: []string{
				`spec: Required value: must be specified`,
			},
		},
		"Response": {
			reason: "SpaceQuery must not have a response",
			query: SpaceQuery{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       &QuerySpec{},
				Response:   &QueryResponse{},
			},
			wantErrs: []string{
				`response: Invalid value: "{}": must not be specified`,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.query.Default()
			errs := tt.query.ValidateCreate()

			var errStrings []string
			for _, err := range errs {
				errStrings = append(errStrings, err.Error())
			}

			if diff := cmp.Diff(tt.wantErrs, errStrings); diff != "" {
				t.Errorf("\n%s\nValidateCreate(...): -want errs, +got errs:\n%s", tt.reason, diff)
			}
		})
	}
}

func TestGroupQueryValidateCreate(t *testing.T) {
	tests := map[string]struct {
		reason   string
		query    GroupQuery
		wantErrs []string
	}{
		"NoNamespace": {
			reason: "GroupQuery is namespace scoped and must have a namespace",
			query: GroupQuery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "",
				},
				Spec: &QuerySpec{},
			},
			wantErrs: []string{
				`metadata.namespace: Required value`,
			},
		},
		"DifferentNamespace": {
			reason: "GroupQuery's namespace must match the namespace in the spec.filter.controlPlane.namespace field",
			query: GroupQuery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						Filter: QueryTopLevelFilter{
							ControlPlane: QueryFilterControlPlane{
								Namespace: "bar",
							},
						},
					},
				},
			},
			wantErrs: []string{
				`spec.filter.controlPlane.namespace: Invalid value: "bar": must match .metadata.namespace`,
			},
		},
		"Name": {
			reason: "GroupQuery's name does not matter",
			query: GroupQuery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						Filter: QueryTopLevelFilter{
							ControlPlane: QueryFilterControlPlane{
								Name: "foo",
							},
						},
					},
				},
			},
		},
		"NoName": {
			reason: "GroupQuery name is defaulted",
			query: GroupQuery{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec: &QuerySpec{},
			},
		},
		"NoSpec": {
			reason: "GroupQuery must have a spec",
			query: GroupQuery{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
			},
			wantErrs: []string{
				`spec: Required value: must be specified`,
			},
		},
		"Response": {
			reason: "GroupQuery must not have a response",
			query: GroupQuery{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec:     &QuerySpec{},
				Response: &QueryResponse{},
			},
			wantErrs: []string{
				`response: Invalid value: "{}": must not be specified`,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.query.Default()
			errs := tt.query.ValidateCreate()

			var errStrings []string
			for _, err := range errs {
				errStrings = append(errStrings, err.Error())
			}

			if diff := cmp.Diff(tt.wantErrs, errStrings); diff != "" {
				t.Errorf("\n%s\nValidateCreate(...): -want errs, +got errs:\n%s", tt.reason, diff)
			}
		})
	}
}

func TestQueryValidateCreate(t *testing.T) {
	tests := map[string]struct {
		reason   string
		query    Query
		wantErrs []string
	}{
		"NoNamespace": {
			reason: "Query is namespace scoped and must have a namespace",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "",
				},
				Spec: &QuerySpec{},
			},
			wantErrs: []string{
				`metadata.namespace: Required value`,
				`spec.filter.name: Required value: must specify a namespace if specifying a name`,
			},
		},
		"DifferentNamespace": {
			reason: "Query's namespace must match the namespace in the spec.filter.controlPlane.namespace field",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						Filter: QueryTopLevelFilter{
							ControlPlane: QueryFilterControlPlane{
								Namespace: "bar",
							},
						},
					},
				},
			},
			wantErrs: []string{
				`spec.filter.controlPlane.namespace: Invalid value: "bar": must match .metadata.namespace`,
			},
		},
		"DifferentName": {
			reason: "Query's name must match the name in the spec.filter.controlPlane.name field",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						Filter: QueryTopLevelFilter{
							ControlPlane: QueryFilterControlPlane{
								Name: "foo",
							},
						},
					},
				},
			},
			wantErrs: []string{
				`spec.filter.controlPlane.name: Invalid value: "foo": must match .metadata.name`,
			},
		},
		"NoName": {
			reason: "Query name must be specified",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
				},
				Spec: &QuerySpec{},
			},
			wantErrs: []string{
				`metadata.name: Required value: name or generateName is required`,
			},
		},
		"NoSpec": {
			reason: "Query must have a spec",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
			},
			wantErrs: []string{
				`spec: Required value: must be specified`,
			},
		},
		"Response": {
			reason: "Query must not have a response",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec:     &QuerySpec{},
				Response: &QueryResponse{},
			},
			wantErrs: []string{
				`response: Invalid value: "{}": must not be specified`,
			},
		},
		"RecursiveAndNonRecursiveRelation": {
			reason: "only one of recursive and non-recursive relations can be specified",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						QueryResources: QueryResources{
							Objects: &QueryObjects{
								Relations: map[string]QueryRelation{
									"events":  {},
									"events+": {},
								},
							},
						},
					},
				},
			},
			wantErrs: []string{
				`spec.objects.relations: Invalid value: "events+": cannot have both "events+" and "events" relations`,
			},
		},
		"SameRelationUnderRecursiveRelation": {
			reason: "(directly) under a recursive relation the same relation must not be specified",
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						QueryResources: QueryResources{
							Objects: &QueryObjects{
								Relations: map[string]QueryRelation{
									"events+": {
										QueryNestedResources: QueryNestedResources{
											QueryResources: QueryResources{
												Objects: &QueryObjects{
													Relations: map[string]QueryRelation{
														"events": {},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErrs: []string{
				`spec.objects.relations[events+].relations: Invalid value: "events": cannot have a "events" relation if the parent has a "events+" relation`,
			},
		},
		"SameRelationFurtherDownUnderRecursiveRelation": {
			query: Query{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec: &QuerySpec{
					QueryTopLevelResources: QueryTopLevelResources{
						QueryResources: QueryResources{
							Objects: &QueryObjects{
								Relations: map[string]QueryRelation{
									"events+": {
										QueryNestedResources: QueryNestedResources{
											QueryResources: QueryResources{
												Objects: &QueryObjects{
													Relations: map[string]QueryRelation{
														"owners": {
															QueryNestedResources: QueryNestedResources{
																QueryResources: QueryResources{
																	Objects: &QueryObjects{
																		Relations: map[string]QueryRelation{
																			"events": {},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.query.Default()
			errs := tt.query.ValidateCreate()

			var errStrings []string
			for _, err := range errs {
				errStrings = append(errStrings, err.Error())
			}

			if diff := cmp.Diff(tt.wantErrs, errStrings); diff != "" {
				t.Errorf("\n%s\nValidateCreate(...): -want errs, +got errs:\n%s", tt.reason, diff)
			}
		})
	}
}
