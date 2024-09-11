// Copyright 2024 Upbound Inc.
// All rights reserved

package resource

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	queryv1alpha2 "github.com/upbound/up-sdk-go/apis/query/v1alpha2"
)

type QueryObject interface {
	client.Object

	GetSpec() *queryv1alpha2.QuerySpec
	SetSpec(spec *queryv1alpha2.QuerySpec) QueryObject
	SetResponse(response *queryv1alpha2.QueryResponse) QueryObject
	GetResponse() *queryv1alpha2.QueryResponse
	DeepCopyQueryObject() QueryObject
}

type SpaceQuery queryv1alpha2.SpaceQuery
type GroupQuery queryv1alpha2.GroupQuery
type Query queryv1alpha2.Query

func (c *SpaceQuery) DeepCopy() *SpaceQuery {
	return (*SpaceQuery)((*queryv1alpha2.SpaceQuery)(c).DeepCopy())
}

func (c *SpaceQuery) DeepCopyInto(cpy *SpaceQuery) {
	(*queryv1alpha2.SpaceQuery)(cpy).DeepCopyInto((*queryv1alpha2.SpaceQuery)(c))
}

func (c *SpaceQuery) DeepCopyObject() runtime.Object {
	return c.DeepCopy()
}

func (c *SpaceQuery) DeepCopyQueryObject() QueryObject {
	return c.DeepCopy()
}

func (c *GroupQuery) DeepCopy() *GroupQuery {
	return (*GroupQuery)((*queryv1alpha2.GroupQuery)(c).DeepCopy())
}

func (c *GroupQuery) DeepCopyInto(cpy *GroupQuery) {
	(*queryv1alpha2.GroupQuery)(cpy).DeepCopyInto((*queryv1alpha2.GroupQuery)(c))
}

func (c *GroupQuery) DeepCopyObject() runtime.Object {
	return c.DeepCopy()
}

func (c *GroupQuery) DeepCopyQueryObject() QueryObject {
	return c.DeepCopy()
}

func (c *Query) DeepCopy() *Query {
	return (*Query)((*queryv1alpha2.Query)(c).DeepCopy())
}

func (c *Query) DeepCopyInto(cpy *Query) {
	(*queryv1alpha2.Query)(cpy).DeepCopyInto((*queryv1alpha2.Query)(c))
}

func (c *Query) DeepCopyObject() runtime.Object {
	return c.DeepCopy()
}

func (c *Query) DeepCopyQueryObject() QueryObject {
	return c.DeepCopy()
}

// GetSpec returns the spec of the query.
func (q *SpaceQuery) GetSpec() *queryv1alpha2.QuerySpec {
	return q.Spec
}

// SetSpec sets the spec of the query.
func (q *SpaceQuery) SetSpec(spec *queryv1alpha2.QuerySpec) QueryObject {
	q.Spec = spec
	return q
}

// SetResponse sets the response of the query.
func (q *SpaceQuery) SetResponse(response *queryv1alpha2.QueryResponse) QueryObject {
	q.Response = response
	return q
}

// GetResponse gets the response of the query.
func (q *SpaceQuery) GetResponse() *queryv1alpha2.QueryResponse {
	return q.Response
}

// GetSpec returns the spec of the query.
func (q *GroupQuery) GetSpec() *queryv1alpha2.QuerySpec {
	return q.Spec
}

// SetSpec sets the spec of the query.
func (q *GroupQuery) SetSpec(spec *queryv1alpha2.QuerySpec) QueryObject {
	q.Spec = spec
	return q
}

// SetResponse sets the response of the query.
func (q *GroupQuery) SetResponse(response *queryv1alpha2.QueryResponse) QueryObject {
	q.Response = response
	return q
}

// GetResponse gets the response of the query.
func (q *GroupQuery) GetResponse() *queryv1alpha2.QueryResponse {
	return q.Response
}

// GetSpec returns the spec of the query.
func (q *Query) GetSpec() *queryv1alpha2.QuerySpec {
	return q.Spec
}

// SetSpec sets the spec of the query.
func (q *Query) SetSpec(spec *queryv1alpha2.QuerySpec) QueryObject {
	q.Spec = spec
	return q
}

// SetResponse sets the response of the query.
func (q *Query) SetResponse(response *queryv1alpha2.QueryResponse) QueryObject {
	q.Response = response
	return q
}

// GetResponse gets the response of the query.
func (q *Query) GetResponse() *queryv1alpha2.QueryResponse {
	return q.Response
}

func init() {
	SchemeBuilder.Register(&SpaceQuery{}, &GroupQuery{}, &Query{})
}
