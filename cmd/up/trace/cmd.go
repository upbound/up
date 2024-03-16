// Copyright 2024 Upbound Inc
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

package trace

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	queryv1alpha1 "github.com/upbound/up-sdk-go/apis/query/v1alpha1"
	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/cmd/up/query/resource"
	"github.com/upbound/up/internal/upbound"
)

type Cmd struct {
	ControlPlane string `short:"c" long:"controlplane" env:"UPBOUND_CONTROLPLANE" description:"Controlplane to query"`
	Group        string `short:"g" long:"group" env:"UPBOUND_GROUP" description:"Group to query"`
	Namespace    string `short:"n" long:"namespace" env:"UPBOUND_NAMESPACE" description:"Namespace of objects to query (defaults to all namespaces)"`
	AllGroups    bool   `short:"A" name:"all-groups" help:"Query in all groups."`

	// positional arguments
	Resources []string `arg:"" help:"Type(s) (resource, singular or plural, category, short-name) and names: TYPE[.GROUP][,TYPE[.GROUP]...] [NAME ...] | TYPE[.GROUP]/NAME .... If no resource is specified, all resources are queried, but --all-resources must be specified."`

	Flags upbound.Flags `embed:""`
}

func (c *Cmd) Help() string {
	return `Examples:
  # Trace all buckets.
  up alpha trace buckets
        
  # Trace all Crossplane claims.
  up alpha trace claims

  # Trace buckets and vpcs.
  up alpha trace buckets,vpc

  # Trace the buckets prod and staging.
  up alpha trace buckets prod staging

  # Trace the bucket prod and the vpc default.
  up alpha trace bucket/prod vpc/default 
`
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	return nil
}

func (c *Cmd) Run(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context) error { // nolint:gocyclo // TODO: split up
	// create client
	kubeconfig, ns, err := upCtx.Profile.GetSpaceKubeConfig()
	if err != nil {
		return err
	}
	if c.Group == "" {
		if !c.AllGroups {
			c.Group = ns
		}
	}
	kc, err := client.New(kubeconfig, client.Options{Scheme: queryScheme})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// parse positional arguments
	tgns, errs := query.ParseTypesAndNames(c.Resources...)
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}
	gkNames, categoryNames := query.SplitGroupKindAndCategories(tgns)

	// create query template depending on the scope
	var queryObject resource.QueryObject
	switch {
	case c.Group != "" && c.ControlPlane != "":
		queryObject = &resource.Query{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
				Name:      c.ControlPlane,
			},
		}
	case c.Group != "":
		queryObject = &resource.GroupQuery{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
			},
		}
	default:
		queryObject = &resource.SpaceQuery{}
	}

	poll := func(gkns query.GroupKindNames, cns query.CategoryNames) ([]queryv1alpha1.QueryResponseObject, error) {
		var querySpecs []*queryv1alpha1.QuerySpec
		for gk, names := range gkns {
			if len(names) == 0 {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace}, gk, nil)
				querySpecs = append(querySpecs, query)
				continue
			}
			for _, name := range names {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace, Name: name}, gk, nil)
				querySpecs = append(querySpecs, query)
			}
		}
		for cat, names := range cns {
			catList := []string{cat}
			if cat == query.AllCategory {
				catList = nil
			}
			if len(names) == 0 {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace}, metav1.GroupKind{}, catList)
				querySpecs = append(querySpecs, query)
				continue
			}
			for _, name := range names {
				query := createQuerySpec(types.NamespacedName{Namespace: c.Namespace, Name: name}, metav1.GroupKind{}, catList)
				querySpecs = append(querySpecs, query)
			}
		}

		var objs []queryv1alpha1.QueryResponseObject
		for _, spec := range querySpecs {
			var cursor string
			var page int
			for {
				spec := spec.DeepCopy()
				spec.QueryTopLevelResources.QueryResources.Page.Cursor = cursor
				query := queryObject.DeepCopyQueryObject().SetSpec(spec)

				if err := kc.Create(ctx, query); err != nil {
					return nil, fmt.Errorf("%T request failed: %w", query, err)
				}
				resp := query.GetResponse()
				objs = append(objs, resp.Objects...)

				// do paging
				cursor = resp.Cursor.Next
				page++
				if cursor == "" {
					break
				}
			}
		}

		return objs, nil
	}

	fetch := func(id string) (*unstructured.Unstructured, error) {
		query := queryObject.DeepCopyQueryObject().SetSpec(&queryv1alpha1.QuerySpec{
			QueryTopLevelResources: queryv1alpha1.QueryTopLevelResources{
				Filter: queryv1alpha1.QueryTopLevelFilter{
					IDs: []string{id},
				},
				QueryResources: queryv1alpha1.QueryResources{
					Objects: &queryv1alpha1.QueryObjects{
						ID:           true,
						ControlPlane: true,
						Object: &queryv1alpha1.JSON{
							Object: true,
						},
					},
				},
			},
		})

		if err := kc.Create(ctx, query); err != nil {
			return nil, fmt.Errorf("failed to SpaceQuery request: %w", err)
		}

		if len(query.GetResponse().Objects) == 0 {
			return nil, fmt.Errorf("not found Object: %s", id)
		}

		return &unstructured.Unstructured{Object: query.GetResponse().Objects[0].Object.Object}, nil
	}

	app := NewApp("upbound trace", c.Resources, gkNames, categoryNames, poll, fetch)
	return app.Run(ctx)
}

func createQuerySpec(obj types.NamespacedName, gk metav1.GroupKind, categories []string) *queryv1alpha1.QuerySpec {
	return &queryv1alpha1.QuerySpec{
		QueryTopLevelResources: queryv1alpha1.QueryTopLevelResources{
			Filter: queryv1alpha1.QueryTopLevelFilter{
				QueryFilter: queryv1alpha1.QueryFilter{
					Kind:       gk.Kind,
					Group:      gk.Group,
					Namespace:  obj.Namespace,
					Name:       obj.Name,
					Categories: categories,
				},
			},
			QueryResources: queryv1alpha1.QueryResources{
				Limit:  500,
				Cursor: true,
				Objects: &queryv1alpha1.QueryObjects{
					ID:           true,
					ControlPlane: true,
					Object: &queryv1alpha1.JSON{
						Object: map[string]interface{}{
							"kind":       true,
							"apiVersion": true,
							"metadata": map[string]interface{}{
								"creationTimestamp": true,
								"deletionTimestamp": true,
								"name":              true,
								"namespace":         true,
							},
							"status": map[string]interface{}{
								"conditions": true,
							},
						},
					},
					Relations: map[string]queryv1alpha1.QueryRelation{
						"events": {
							QueryNestedResources: queryv1alpha1.QueryNestedResources{
								QueryResources: queryv1alpha1.QueryResources{
									Objects: &queryv1alpha1.QueryObjects{
										ID:           true,
										ControlPlane: true,
										Object: &queryv1alpha1.JSON{
											Object: map[string]interface{}{
												"lastTimestamp": true,
												"message":       true,
												"count":         true,
												"type":          true,
											},
										},
									},
								},
							},
						},
						"resources+": {
							QueryNestedResources: queryv1alpha1.QueryNestedResources{
								QueryResources: queryv1alpha1.QueryResources{
									Limit: 10000,
									Objects: &queryv1alpha1.QueryObjects{
										ID:           true,
										ControlPlane: true,
										Object: &queryv1alpha1.JSON{
											Object: map[string]interface{}{
												"kind":       true,
												"apiVersion": true,
												"metadata": map[string]interface{}{
													"creationTimestamp": true,
													"deletionTimestamp": true,
													"name":              true,
													"namespace":         true,
												},
												"status": map[string]interface{}{
													"conditions": true,
												},
											},
										},
										Relations: map[string]queryv1alpha1.QueryRelation{
											"events": {
												QueryNestedResources: queryv1alpha1.QueryNestedResources{
													QueryResources: queryv1alpha1.QueryResources{
														Objects: &queryv1alpha1.QueryObjects{
															ID:           true,
															ControlPlane: true,
															Object: &queryv1alpha1.JSON{
																Object: map[string]interface{}{
																	"lastTimestamp": true,
																	"message":       true,
																	"count":         true,
																	"type":          true,
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
}
