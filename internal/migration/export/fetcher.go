package export

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
	FetchResources(ctx context.Context) ([]unstructured.Unstructured, error)
}

type PagedResourceFetcher struct {
	kube     dynamic.Interface
	gvr      schema.GroupVersionResource
	pageSize int64
}

func NewPagedResourceFetcher(kube dynamic.Interface, gvr schema.GroupVersionResource) *PagedResourceFetcher {
	return &PagedResourceFetcher{
		kube:     kube,
		gvr:      gvr,
		pageSize: defaultPageSize,
	}
}

func (e *PagedResourceFetcher) FetchResources(ctx context.Context) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured

	var continueToken string
	for {
		l, err := e.kube.Resource(e.gvr).List(ctx, v1.ListOptions{
			Limit:    e.pageSize,
			Continue: continueToken,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "cannot list %q resources", e.gvr.GroupResource())
		}
		for _, r := range l.Items {
			resources = append(resources, r)
		}
		continueToken = l.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return resources, nil
}
