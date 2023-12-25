package exporter

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
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
			if !e.shouldSkip(r) {
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

func (e *UnstructuredFetcher) shouldSkip(r unstructured.Unstructured) bool {
	// Filter out namespaces that are not in the scope.
	// - If the resource is a Namespace and its name is not in the scope, skip it.
	// - If the resource is namespaced and its namespace is in the scope, skip it.
	if r.GetKind() == "Namespace" && !e.namespaceInScope(r.GetName()) ||
		r.GetNamespace() != "" && !e.namespaceInScope(r.GetNamespace()) {
		return true
	}

	if r.GetKind() == "ConfigMap" && r.GetName() == "kube-root-ca.crt" {
		// This is cluster-specific and should not be exported.
		return true
	}

	if r.GetLabels() != nil && r.GetLabels()["app.kubernetes.io/managed-by"] == "Helm" {
		// We don't want to export Helm resources. They need to be installed
		// to the target cluster again using Helm.
		// A typical example is the TLS secrets for Crossplane.
		return true
	}

	if r.GetKind() == "Secret" {
		paved := fieldpath.Pave(r.Object)
		s, _ := paved.GetString("type")
		if s == "helm.sh/release.v1" {
			// We don't want to export Helm secrets.
			return true
		}
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
			// We don't want to export resources that are owned by the package
			// manager. They will be installed to the target cluster again
			// using the package manager.
			// A typical example is the TLS secrets for providers.
			return true
		}
	}

	return false
}
