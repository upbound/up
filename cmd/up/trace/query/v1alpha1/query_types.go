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

package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A QuerySpec specifies what to query.
type QuerySpec struct {
	QueryTopLevelResources `json:",inline"`
}

// A QueryTopLevelFilter specifies how to filter top level objects. In contrast
// to QueryFilter, it also specifies which controlplane to query.
type QueryTopLevelFilter struct {
	// controlPlane specifies which controlplanes to query. If empty, all
	// controlplanes are queried in the given scope.
	ControlPlane QueryFilterControlPlane `json:"controlPlane,omitempty"`

	//	# ids: ["id1","id2"] # to get objects explicitly by id.
	IDs []string `json:"ids,omitempty"`

	QueryFilter `json:",inline"`
}

// A QueryFilter specifies what to filter.
type QueryFilter struct {
	// namespace is the namespace WITHIN a controlplane to query. If empty,
	// all namespaces are queried in the given scope.
	Namespace string `json:"namespace,omitempty"`
	// name is the name of the object to query. If empty, all objects are queried
	// in the given scope.
	Name string `json:"name,omitempty"`
	// group is the API group to query. If empty, all groups are queried in the
	// given scope.
	Group string `json:"group,omitempty"`
	// kind is the API kind to query. If empty, all kinds are queried in the
	// given scope. The kind is case-insensitive. The kind also matches plural
	// resources.
	Kind string `json:"kind,omitempty"`
	// categories is a list of categories to query. If empty, all categories are
	// queried in the given scope.
	// Examples: all, managed, composite, claim
	Categories []string `json:"categories,omitempty"`
	// conditions is a list of conditions to query. If empty, all conditions are
	// queried in the given scope.
	Conditions []QueryCondition `json:"conditions,omitempty"`
	// owners is a list of owners to query. An object matches if it has at least
	// one owner in the list.
	Owners []QueryOwner `json:"owners,omitempty"`
	// sql is a SQL query to query. If empty, all objects are queried in the
	// given scope.
	//
	// The current object can be referenced by the alias "o".
	//
	// WARNING: The where clause is highly dependent on the database
	// schema and might change at any time. The schema is not documented.
	SQL string `json:"sql,omitempty"`
}

type QueryFilterControlPlane struct {
	// name is the name of the controlplane to query. If empty, all controlplanes
	// are queried in the given scope.
	Name string `json:"name,omitempty"`
	// namespace is the namespace of the controlplane to query. If empty, all
	// namespaces are queried in the given scope.
	Namespace string `json:"namespace,omitempty"`
}

// A QueryCondition specifies how to query a condition.
type QueryCondition struct {
	// type is the type of condition to query.
	// Examples: Ready, Synced
	//
	// +kubebuilder:validation:Required
	Type string `json:"type"`
	// status is the status of condition to query. This is either True, False
	// or Unknown.
	//
	// +kubebuilder:validation:Required
	Status string `json:"status"`
}

// A QueryOwner specifies how to query by owner.
type QueryOwner struct {
	// name is the name of the owner to match.
	Group string `json:"group,omitempty"`
	// kind is the kind of the owner to match.
	Kind string `json:"kind,omitempty"`
	// uid is the uid of the owner to match.
	UID string `json:"uid,omitempty"`
}

type Direction string

const (
	Ascending  Direction = "Asc"
	Descending Direction = "Desc"
)

// A QueryOrder specifies how to order. Exactly one of the fields must be set.
type QueryOrder struct {
	// creationTimestamp specifies how to order by creation timestamp.
	//
	// +kubebuilder:validation:Enum=Asc;Desc
	CreationTimestamp Direction `json:"creationTimestamp,omitempty"`

	// name specifies how to order by name.
	//
	// +kubebuilder:validation:Enum=Asc;Desc
	Name Direction `json:"name,omitempty"`

	// namespace specifies how to order by namespace.
	//
	// +kubebuilder:validation:Enum=Asc;Desc
	Namespace Direction `json:"namespace,omitempty"`

	// kind specifies how to order by kind.
	//
	// +kubebuilder:validation:Enum=Asc;Desc
	Kind Direction `json:"kind,omitempty"`

	// group specifies how to order by group.
	//
	// +kubebuilder:validation:Enum=Asc;Desc
	Group Direction `json:"group,omitempty"`

	// controlPlane specifies how to order by controlplane.
	ControlPlane Direction `json:"cluster"`
}

// A QueryPage specifies how to page.
type QueryPage struct {
	// first is the number of the first object to return relative to the cursor.
	// If neither first nor cursor is specified, objects are returned from the
	// beginning.
	First int `json:"first,omitempty"`
	// cursor is the cursor of the first object to return. This value is opaque,
	// the format cannot be relied on. It is returned by the server in the
	// response to a previous query. If neither first nor cursor is specified,
	// objects are returned from the beginning.
	//
	// Note that cursor values are not stable under different orderings.
	Cursor string `json:"cursor,omitempty"`
}

// A QueryTopLevelResources specifies how to return top level objects.
type QueryTopLevelResources struct {
	QueryResources `json:",inline"`

	// filter specifies how to filter the returned objects.
	Filter QueryTopLevelFilter `json:"filter,omitempty"`
}

type QueryNestedResources struct {
	QueryResources `json:",inline"`

	// filter specifies how to filter the returned objects.
	Filter QueryFilter `json:"filter,omitempty"`
}

type QueryResources struct {
	// count specifies whether to return the number of objects. Note that
	// computing the count is expensive and should only be done if necessary.
	// Count is the remaining objects that match the query after paging.
	Count bool `json:"count,omitempty"`

	// objects specifies how to return the objects.
	Objects *QueryObjects `json:"objects,omitempty"`

	// order specifies how to order the returned objects. The first element
	// specifies the primary order, the second element specifies the secondary,
	// etc.
	Order []QueryOrder `json:"order,omitempty"`

	// limit is the maximal number of objects to return. Defaulted to 100.
	//
	// Note that a limit in a relation subsumes all the children of all parents,
	// i.e. a small limit only makes sense if there is only a single parent,
	// e.g. selected via spec.IDs.
	Limit int `json:"limit,omitempty"`

	// Page specifies how to page the returned objects.
	Page QueryPage `json:"page,omitempty"`

	// Cursor specifies the cursor of the first object to return. This value is
	// opaque and is only valid when passed into spec.page.cursor in a subsequent
	// query. The format of the cursor might change between releases.
	Cursor bool `json:"cursor,omitempty"`
}

// A QueryObjects specifies how to return objects.
type QueryObjects struct {
	// id specifies whether to return the id of the object. The id is opaque,
	// i.e. the format is undefined. It's only valid for comparison within the
	// response and as part of the spec.ids field in immediately following queries.
	// The format of the id might change between releases.
	ID bool `json:"id,omitempty"`

	// mutablePath specifies whether to return the mutable path of the object,
	// i.e. the path to the object in the controlplane Kubernetes API.
	MutablePath bool `json:"mutablePath,omitempty"`

	// controlPlane specifies that the controlplane name and namespace of the
	// object should be returned.
	ControlPlane bool `json:"controlPlane,omitempty"`

	// object specifies how to return the object, i.e. a sparse skeleton of
	// fields. A value of true means that all descendants of that field should
	// be returned. Other primitive values are not allowed. If the type of
	// a field does not match the schema (e.g. an array instead of an object),
	// the field is ignored.
	//
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Object *JSON `json:"object,omitempty"`

	// relations specifies which relations to query and what to return.
	// Relation names are predefined strings relative to the release of
	// Spaces.
	//
	// Examples: owners, descendants, resources, events, or their transitive
	// equivalents owners+, descendants+, resources+.
	Relations map[string]QueryRelation `json:"relations,omitempty"`
}

// A QueryRelation specifies how to return objects in a relation.
type QueryRelation struct {
	QueryNestedResources `json:",inline"`
}

// SpaceQuery represents a query against all controlplanes.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SpaceQuery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec     *QuerySpec     `json:"spec,omitempty"`
	Response *QueryResponse `json:"response,omitempty"`
}

// GroupQuery represents a query against a group of controlplanes.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GroupQuery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec     *QuerySpec     `json:"spec,omitempty"`
	Response *QueryResponse `json:"response,omitempty"`
}

// Query represents a query against one controlplane, the one with the same
// name and namespace as the query.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Query struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec     *QuerySpec     `json:"spec,omitempty"`
	Response *QueryResponse `json:"response,omitempty"`
}

func (q *SpaceQuery) GetSpec() *QuerySpec {
	return q.Spec
}

func (q *SpaceQuery) SetSpec(spec *QuerySpec) {
	q.Spec = spec
}

func (q *SpaceQuery) SetResponse(response *QueryResponse) {
	q.Response = response
}

func (q *GroupQuery) GetSpec() *QuerySpec {
	return q.Spec
}

func (q *GroupQuery) SetSpec(spec *QuerySpec) {
	q.Spec = spec
}

func (q *GroupQuery) SetResponse(response *QueryResponse) {
	q.Response = response
}

func (q *Query) GetSpec() *QuerySpec {
	return q.Spec
}

func (q *Query) SetSpec(spec *QuerySpec) {
	q.Spec = spec
}

func (q *Query) SetResponse(response *QueryResponse) {
	q.Response = response
}

var (
	SpacesQueryKind = reflect.TypeOf(SpaceQuery{}).Name()
	GroupQueryKind  = reflect.TypeOf(GroupQuery{}).Name()
	QueryKind       = reflect.TypeOf(Query{}).Name()
)

func init() {
	SchemeBuilder.Register(&SpaceQuery{}, &GroupQuery{}, &Query{})
}
