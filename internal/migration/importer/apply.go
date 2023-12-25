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

type ResourceTransformer func(*unstructured.Unstructured)

type TransformingResourceApplier struct {
	dynamicClient  dynamic.Interface
	resourceMapper meta.RESTMapper

	transformers []ResourceTransformer
}

func NewTransformingResourceApplier(dynamicClient dynamic.Interface, resourceMapper meta.RESTMapper, transformers ...ResourceTransformer) *TransformingResourceApplier {
	return &TransformingResourceApplier{
		dynamicClient:  dynamicClient,
		resourceMapper: resourceMapper,
		transformers:   transformers,
	}
}

func (a *TransformingResourceApplier) ApplyResources(ctx context.Context, resources []unstructured.Unstructured) error {
	for _, r := range resources {
		if r.GetName() == "kube-root-ca.crt" {
			// TODO: This is a hack to avoid applying the kube-root-ca.crt ConfigMap.
			continue
		}
		if r.GetLabels() != nil && r.GetLabels()["app.kubernetes.io/managed-by"] == "Helm" {
			// TODO: This is a hack to avoid applying Helm resources.
			continue
		}
		if r.GetOwnerReferences() != nil {
			ownedByPackageManager := false
			for _, or := range r.GetOwnerReferences() {
				if or.APIVersion == "pkg.crossplane.io/v1" {
					ownedByPackageManager = true
					break
				}
			}
			if ownedByPackageManager {
				// TODO: This is a hack to avoid applying resources owned by the package manager.
				continue
			}
		}
		for _, t := range a.transformers {
			t(&r)
		}

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
