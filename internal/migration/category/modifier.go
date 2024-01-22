package category

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

type Modifier interface {
	ModifyResources(ctx context.Context, category string, modify func(*unstructured.Unstructured) error) (int, error)
}

type APICategoryModifier struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
}

func NewAPICategoryModifier(dyn dynamic.Interface, dis discovery.DiscoveryInterface) *APICategoryModifier {
	return &APICategoryModifier{
		dynamicClient:   dyn,
		discoveryClient: dis,
	}
}

func (a *APICategoryModifier) ModifyResources(ctx context.Context, category string, modify func(*unstructured.Unstructured) error) (int, error) {
	count := 0
	apiLists, err := a.discoveryClient.ServerPreferredResources()
	if err != nil {
		return 0, errors.Wrap(err, "cannot get server preferred resources")
	}
	for _, al := range apiLists {
		for _, r := range al.APIResources {
			if contains(r.Categories, category) {
				gvr := schema.GroupVersionResource{
					Group:    r.Group,
					Version:  r.Version,
					Resource: r.Name,
				}
				ul, err := a.dynamicClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
				if err != nil {
					return 0, errors.Wrapf(err, "cannot list resources %s", r.Name)
				}
				for _, item := range ul.Items {
					if err = retry.OnError(retry.DefaultRetry, resource.IsAPIError, func() error {
						u, err := a.dynamicClient.Resource(gvr).Namespace(item.GetNamespace()).Get(ctx, item.GetName(), metav1.GetOptions{})
						if err != nil {
							return errors.Wrapf(err, "cannot get resource %s/%s", item.GetKind(), item.GetName())
						}
						if err = modify(u); err != nil {
							return err
						}
						_, err = a.dynamicClient.Resource(gvr).Namespace(u.GetNamespace()).Update(ctx, u, metav1.UpdateOptions{})
						if err != nil {
							return errors.Wrapf(err, "cannot update resource %s/%s", u.GetKind(), u.GetName())
						}
						count++
						return nil
					}); err != nil {
						return 0, errors.Wrapf(err, "cannot modify resource %s/%s", item.GetKind(), item.GetName())
					}
				}
			}
		}
	}
	return count, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
