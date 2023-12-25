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
