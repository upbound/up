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
	nativeKinds     = map[string]string{}
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

		"Binding":                          "bindings",
		"CSIDriver":                        "csidrivers",
		"CSINode":                          "csinodes",
		"CSIStorageCapacity":               "csistoragecapacities",
		"CertificateSigningRequest":        "certificatesigningrequests",
		"ClusterTrustBundle":               "clustertrustbundles",
		"ComponentStatus":                  "componentstatuses",
		"ControllerRevision":               "controllerrevisions",
		"ConversionReview":                 "conversionreviews",
		"CronJob":                          "cronjobs",
		"DaemonSet":                        "daemonsets",
		"DeploymentRollback":               "deploymentrollbacks",
		"Eviction":                         "evictions",
		"HorizontalPodAutoscaler":          "horizontalpodautoscalers",
		"IPAddress":                        "ipaddresses",
		"Ingress":                          "ingresses",
		"IngressClass":                     "ingressclasses",
		"Job":                              "jobs",
		"LimitRange":                       "limitranges",
		"LocalSubjectAccessReview":         "localsubjectaccessreviews",
		"MutatingWebhookConfiguration":     "mutatingwebhookconfigurations",
		"NetworkPolicy":                    "networkpolicies",
		"PersistentVolume":                 "persistentvolumes",
		"PersistentVolumeClaim":            "persistentvolumeclaims",
		"PodSchedulingContext":             "podscchedulingcontexts",
		"PodStatusResult":                  "podstatusresults",
		"PodTemplate":                      "podtemplates",
		"RangeAllocation":                  "rangeallocations",
		"ReplicationController":            "replicationcontrollers",
		"ResourceClaim":                    "resourceclaims",
		"ResourceClaimTemplate":            "resourceclaimtemplates",
		"ResourceClass":                    "resourceclasses",
		"ResourceQuota":                    "resourcequotas",
		"RuntimeClass":                     "runtimeclasses",
		"SelfSubjectAccessReview":          "selfsubjectaccessreviews",
		"SelfSubjectReview":                "selfsubjectreviews",
		"SelfSubjectRulesReview":           "selfsubjectrulesreviews",
		"SerializedReference":              "serializedreferences",
		"ServiceCIDR":                      "servicecidrs",
		"StatefulSet":                      "statefulsets",
		"StorageClass":                     "storageclasses",
		"StorageVersion":                   "storageversions",
		"SubjectAccessReview":              "subjectaccessreviews",
		"TokenRequest":                     "tokenrequests",
		"TokenReview":                      "tokenreviews",
		"ValidatingAdmissionPolicy":        "validatingadmissionpolicies",
		"ValidatingAdmissionPolicyBinding": "validatingadmissionpolicybindings",
		"VolumeAttachment":                 "volumeattachments",
		"VolumeAttributesClass":            "volumeattributesclasses",
	}
)

func init() {
	for k, v := range nativeResources {
		nativeKinds[v] = k
	}
}
