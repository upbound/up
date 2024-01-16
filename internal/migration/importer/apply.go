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
	ApplyResources(ctx context.Context, resources []unstructured.Unstructured, applyStatus bool) error
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

func (a *UnstructuredResourceApplier) ApplyResources(ctx context.Context, resources []unstructured.Unstructured, applyStatus bool) error {
	for i := range resources {
		rm, err := a.resourceMapper.RESTMapping(resources[i].GroupVersionKind().GroupKind(), resources[i].GroupVersionKind().Version)
		if err != nil {
			return errors.Wrap(err, "cannot get REST mapping for resource")
		}

		rs := resources[i].DeepCopy()
		_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).Apply(ctx, resources[i].GetName(), &resources[i], v1.ApplyOptions{
			FieldManager: "up-controlplane-migrator",
			Force:        true,
		})
		if err != nil {
			return errors.Wrapf(err, "cannot apply resource %q", resources[i].GetName())
		}

		if !applyStatus {
			continue
		}
		_, err = a.dynamicClient.Resource(rm.Resource).Namespace(resources[i].GetNamespace()).ApplyStatus(ctx, rs.GetName(), rs, v1.ApplyOptions{
			FieldManager: "up-controlplane-migrator",
			Force:        true,
		})
		if err != nil {
			return errors.Wrapf(err, "cannot apply status subresource %q", resources[i].GetName())
		}
	}

	return nil
}
