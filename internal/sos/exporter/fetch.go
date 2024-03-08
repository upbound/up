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

package exporter

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	defaultPageSize = 500
)

type ResourceFetcher interface {
	FetchResources(ctx context.Context, gvr schema.GroupVersionResource) ([]unstructured.Unstructured, error)
}

type UnstructuredFetcher struct {
	kube     dynamic.Interface
	pageSize int64
}

func NewUnstructuredFetcher(kube dynamic.Interface, opts Options) *UnstructuredFetcher {

	return &UnstructuredFetcher{
		kube:     kube,
		pageSize: defaultPageSize,
	}
}

func (e *UnstructuredFetcher) FetchResources(ctx context.Context, gvr schema.GroupVersionResource) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured

	continueToken := ""
	for {
		l, err := e.kube.Resource(gvr).List(ctx, v1.ListOptions{
			Limit:    e.pageSize,
			Continue: continueToken,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "cannot list %q resources", gvr.GroupResource())
		}
		resources = append(resources, l.Items...)
		continueToken = l.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return resources, nil
}
