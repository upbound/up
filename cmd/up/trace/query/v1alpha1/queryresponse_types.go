// Copyright 2023 Upbound Inc
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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A QueryResponse is returned by the query server as response to a Query.
type QueryResponse struct {
	// warnings is a list of warnings that occurred while processing the query.
	// The query is still executed, but these warnings indicate problems. It
	// is recommended to print these to the user.
	Warnings []string `json:"warnings,omitempty"`

	QueryResponseObjects `json:",inline"`
}

type QueryResponseObjects struct {
	// cursor is cursor to the next page of results.
	Cursor *QueryResponseCursor `json:"cursor,omitempty"`

	// objects is the list of objects returned by the query.
	Objects []QueryResponseObject `json:"objects,omitempty"`

	// count is the total number of objects that match the query.
	Count *int `json:"count,omitempty"`

	// incomplete is true if the query was (potentially) limited before all
	// matching objects. If a non-empty spec.page.cursor has been used, or
	// objects have been skipped through non-zero spec.page.first, this value
	// is always true.
	Incomplete bool `json:"incomplete,omitempty"`
}

type QueryResponseCursor struct {
	// cursor is the cursor to the next page of results. If empty, there are no more,
	Next string `json:"next"`
	// page is the page number of the cursor.
	Page int `json:"page"`
	// pageSize is the number of objects per page, i.e. the limit of the query.
	PageSize int `json:"pageSize"`
	// position is the position of the first object in the list of matching objects
	// at the time the first cursor was created. Due to creation and deletion of
	// objects before the cursor this value might be outdated.
	Position int `json:"position"`
}

type QueryResponseObject struct {
	// id indentifies the object. The id is opaque, i.e. the format is
	// undefined. It's only valid for comparison within the response and as part
	// of the spec.ids field in immediately following queries. The format of the
	// id might change between releases.
	ID string `json:"id,omitempty"`

	// mutablePath is the mutable path of the object, i.e. the path to the
	// object in the controlplane Kubernetes API.
	MutablePath *QueryResponseMutablePath `json:"mutablePath,omitempty"`

	// controlPlane is the name and namespace of the object.
	ControlPlane *QueryResponseControlPlane `json:"controlPlane,omitempty"`

	// object is the sparse representation of the object.
	Object *JSONObject `json:"object,omitempty"`

	// errors is the list of errors that occurred while processing the object.
	Errors []string `json:"$errors,omitempty"`

	// relations is the list of objects related to the object.
	Relations map[string]QueryResponseRelation `json:"relations,omitempty"`
}

type QueryResponseMutablePath struct {
	// basePath is the base URL of the controlplane, i.e. the Kubernetes API
	// endpoint.
	BasePath string `json:"basePath,omitempty"`

	metav1.GroupVersionResource `json:",inline"`
}

type QueryResponseControlPlane struct {
	// name is the name of the controlplane of the object.
	Name string `json:"name,omitempty"`
	// namespace is the namespace of the controlplane of the object.
	Namespace string `json:"namespace,omitempty"`
}

type QueryResponseRelation struct {
	QueryResponseObjects `json:",inline"`
}
