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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	queryv1alpha1 "github.com/upbound/up/cmd/up/trace/query/v1alpha1"
)

func init() {
	runtime.Must(queryv1alpha1.AddToScheme(scheme.Scheme))
}

var nativeKinds = sets.New("poddisruptionbudgets", "rolebindings", "endpoints", "apiservices", "pods", "events",
	"serviceaccounts", "flowschemas", "nodes", "prioritylevelconfigurations", "leases", "validatingwebhookconfigurations",
	"priorityclasses", "namespaces", "configmaps", "customresourcedefinitions", "clusterrolebindings", "secrets",
	"services", "endpointslices", "replicasets", "deployments", "roles", "clusterroles",
	"poddisruptionbudget", "rolebinding", "endpoint", "apiservice", "pod", "event",
	"serviceaccount", "flowschema", "node", "prioritylevelconfiguration", "lease", "validatingwebhookconfiguration",
	"priorityclass", "namespace", "configmap", "customresourcedefinition", "clusterrolebinding", "secret",
	"service", "endpointslice", "replicaset", "deployment", "role", "clusterrole")

type Cmd struct {
	ControlPlane string `short:"c" long:"controlplane" env:"UPBOUND_CONTROLPLANE" description:"Controlplane to query"`
	Group        string `short:"g" long:"group" env:"UPBOUND_GROUP" description:"Group to query"`
	Namespace    string `short:"n" long:"namespace" env:"UPBOUND_NAMESPACE" description:"Namespace of objects to query"`

	Kind string `arg:"" description:"Kind to trace, accepts the 'KIND[.GROUP][/NAME]' format."`
	Name string `arg:"" description:"Name to trace" optional:""`
}

func (c *Cmd) Help() string {
	return `
Usage:
    trace [options] <resource> [<name>]

The 'trace' command turns water into wine.

Examples:
    trace
        Makes you rich.
`
}

// BeforeApply sets default values for the delete command, before assignment and validation.
func (c *Cmd) BeforeApply() error {
	return nil
}

func (c *Cmd) Run(ctx context.Context) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	kc, err := client.New(cfg, client.Options{})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	poll := func(group, kind, name string) (*queryv1alpha1.QueryResponse, error) {
		if kind == "minion" || kind == "minions" {
			kind = "nodes" // this got renamed ages ago. Etcd path is still "minions".
		}

		var categories []string
		if c.Group == "" && !nativeKinds.Has(kind) {
			categories = append(categories, kind)
			kind = ""
		}

		query := &queryv1alpha1.SpaceQuery{
			Spec: &queryv1alpha1.QuerySpec{
				QueryTopLevelResources: queryv1alpha1.QueryTopLevelResources{
					Filter: queryv1alpha1.QueryTopLevelFilter{
						ControlPlane: queryv1alpha1.QueryFilterControlPlane{
							Name:      c.ControlPlane,
							Namespace: c.Group,
						},
						QueryFilter: queryv1alpha1.QueryFilter{
							Kind:       kind,
							Group:      group,
							Namespace:  c.Namespace,
							Name:       name,
							Categories: categories,
						},
					},
					QueryResources: queryv1alpha1.QueryResources{
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
			},
		}

		if err := kc.Create(ctx, query); err != nil {
			return nil, fmt.Errorf("SpaceQuery request failed: %w", err)
		}

		return query.Response, nil
	}

	fetch := func(id string) (*unstructured.Unstructured, error) {
		query := &queryv1alpha1.SpaceQuery{
			Spec: &queryv1alpha1.QuerySpec{
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
			},
		}

		if err := kc.Create(ctx, query); err != nil {
			return nil, fmt.Errorf("failed to SpaceQuery request: %w", err)
		}

		if len(query.Response.Objects) == 0 {
			return nil, fmt.Errorf("not found Object: %s", id)
		}

		return &unstructured.Unstructured{Object: query.Response.Objects[0].Object.Object}, nil
	}

	group, kind, name, err := getGroupKindName(c.Kind, c.Name)
	if err != nil {
		return err
	}
	app := NewApp("upbound trace", group, kind, name, poll, fetch)
	return app.Run(ctx)
}

func getGroupKindName(kind, name string) (string, string, string, error) {
	// If no kind was provided, error out (should never happen as it's
	// required by Kong)
	if kind == "" {
		return "", "", "", errors.New("invalid kind, must be provided in the 'KIND[.GROUP][/NAME]' format")
	}

	// Split the resource into its components
	ss := strings.Split(kind, "/")
	length := len(ss)

	var gk string
	switch length {
	case 1:
		gk, name = ss[0], name
	case 2:
		// If a name is separately provided, error out
		if name != "" {
			return "", "", "", errors.New("name kind twice, must be provided separately 'KIND[.GROUP] [NAME]' or in the 'TYPE[.GROUP][/NAME]' format")
		}
		gk, name = ss[0], ss[1]
	default:
		return "", "", "", errors.New("invalid kind, must be provided in the 'KIND[.GROUP][/NAME]' format")
	}

	ss = strings.SplitN(gk, ".", 2)
	var group string
	if len(ss) == 1 {
		group, kind = "", ss[0]
	} else {
		group, kind = ss[1], ss[0]
	}

	return group, kind, name, nil
}
