// Copyright 2023 Upbound Inc
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

package importer

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type ResourceApplier interface {
	ApplyResources(ctx context.Context, resources []unstructured.Unstructured) error
}

type UnstructuredResourceApplier struct {
	dynamicClient  dynamic.Interface
	resourceMapper meta.RESTMapper
}

func NewUnstructuredResourceApplier(dynamicClient dynamic.Interface, resourceMapper meta.RESTMapper) *UnstructuredResourceApplier {
	return &UnstructuredResourceApplier{
		dynamicClient:  dynamicClient,
		resourceMapper: resourceMapper,
	}
}

func (a *UnstructuredResourceApplier) ApplyResources(ctx context.Context, resources []unstructured.Unstructured) error {
	for _, r := range resources {
		rm, err := a.resourceMapper.RESTMapping(r.GroupVersionKind().GroupKind(), r.GroupVersionKind().Version)
		if err != nil {
			return errors.Wrap(err, "cannot get REST mapping for resource")
		}

		_, err = a.dynamicClient.Resource(rm.Resource).Namespace(r.GetNamespace()).Apply(ctx, r.GetName(), &r, v1.ApplyOptions{
			FieldManager: "up-controlplane-migrator",
			Force:        true,
		})

		return errors.Wrapf(err, "cannot apply resource %q", r.GetName())
	}

	return nil
}
