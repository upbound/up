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
	FetchResources(ctx context.Context, gvr schema.GroupVersionResource) ([]unstructured.Unstructured, error)
}

var (
	defaultExcludedNamespaces = map[string]struct{}{
		"kube-system":        {},
		"kube-public":        {},
		"kube-node-lease":    {},
		"local-path-storage": {},
	}
)

type UnstructuredFetcher struct {
	kube     dynamic.Interface
	pageSize int64

	includedNamespaces map[string]struct{}
	excludedNamespaces map[string]struct{}
}

func NewUnstructuredFetcher(kube dynamic.Interface, opts Options) *UnstructuredFetcher {
	inc := make(map[string]struct{}, len(opts.IncludedNamespaces))
	for _, ns := range opts.IncludedNamespaces {
		inc[ns] = struct{}{}
	}
	exc := make(map[string]struct{}, len(opts.ExcludedNamespaces))
	for _, ns := range opts.ExcludedNamespaces {
		exc[ns] = struct{}{}
	}

	return &UnstructuredFetcher{
		kube:     kube,
		pageSize: defaultPageSize,

		includedNamespaces: inc,
		excludedNamespaces: exc,
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
		for _, r := range l.Items {
			// Filter in resources that are in the scope of the exporter.
			// - If the resource is a Namespace and its name is in the scope, include it.
			// - If the resource is cluster-scoped but not a Namespace, include it.
			// - If the resource is namespaced and its namespace is in the scope, include it.
			if r.GetKind() == "Namespace" && e.namespaceInScope(r.GetName()) ||
				r.GetNamespace() == "" && r.GetKind() != "Namespace" ||
				r.GetNamespace() != "" && e.namespaceInScope(r.GetNamespace()) {
				resources = append(resources, r)
			}
		}
		continueToken = l.GetContinue()
		if continueToken == "" {
			break
		}
	}

	return resources, nil
}

func (e *UnstructuredFetcher) namespaceInScope(namespace string) bool {
	if len(e.includedNamespaces) > 0 {
		if _, ok := e.includedNamespaces[namespace]; !ok {
			return false
		}
	}

	if _, ok := e.excludedNamespaces[namespace]; ok {
		return false
	}

	return true
}
