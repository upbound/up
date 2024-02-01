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

package query

var (
	nativeKinds = map[string]string{
		"poddisruptionbudgets":            "PodDisruptionBudget",
		"rolebindings":                    "RoleBinding",
		"endpoints":                       "Endpoints",
		"apiservices":                     "APIService",
		"pods":                            "Pod",
		"events":                          "Event",
		"serviceaccounts":                 "ServiceAccount",
		"flowschemas":                     "FlowSchema",
		"nodes":                           "Node",
		"prioritylevelconfigurations":     "PriorityLevelConfiguration",
		"leases":                          "Lease",
		"validatingwebhookconfigurations": "ValidatingWebhookConfiguration",
		"priorityclasses":                 "PriorityClass",
		"namespaces":                      "Namespace",
		"configmaps":                      "ConfigMap",
		"customresourcedefinitions":       "CustomResourceDefinition",
		"clusterrolebindings":             "ClusterRoleBinding",
		"secrets":                         "Secret",
		"services":                        "Service",
		"endpointslices":                  "EndpointSlice",
		"replicasets":                     "ReplicaSet",
		"deployments":                     "Deployment",
		"roles":                           "Role",
		"clusterroles":                    "ClusterRole",
	}

	nativeResources = map[string]string{
		"PodDisruptionBudget":            "poddisruptionbudgets",
		"RoleBinding":                    "rolebindings",
		"Endpoints":                      "endpoints",
		"APIService":                     "apiservices",
		"Pod":                            "pods",
		"Event":                          "events",
		"ServiceAccount":                 "serviceaccounts",
		"FlowSchema":                     "flowschemas",
		"Node":                           "nodes",
		"PriorityLevelConfiguration":     "prioritylevelconfigurations",
		"Lease":                          "leases",
		"ValidatingWebhookConfiguration": "validatingwebhookconfigurations",
		"PriorityClass":                  "priorityclasses",
		"Namespace":                      "namespaces",
		"ConfigMap":                      "configmaps",
		"CustomResourceDefinition":       "customresourcedefinitions",
		"ClusterRoleBinding":             "clusterrolebindings",
		"Secret":                         "secrets",
		"Service":                        "services",
		"EndpointSlice":                  "endpointslices",
		"ReplicaSet":                     "replicasets",
		"Deployment":                     "deployments",
		"Role":                           "roles",
		"ClusterRole":                    "clusterroles",
	}
)
