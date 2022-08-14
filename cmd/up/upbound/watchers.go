package upbound

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/upbound/up/internal/resources"
)

var (
	infoTextFmt                = "%s/%s updated"
	startingComponentWatchText = "Starting components"
)

func watchCustomResource(ctx context.Context, gvr schema.GroupVersionResource, kconfig *rest.Config) error {
	crdClient, err := dynamic.NewForConfig(kconfig)
	if err != nil {
		return err
	}

	crdWatcher, err := crdClient.Resource(gvr).Watch(ctx, metav1.ListOptions{TimeoutSeconds: &watcherTimeout})
	if err != nil {
		return err
	}

	for {
		event, ok := <-crdWatcher.ResultChan()
		if !ok {
			break
		}

		uu, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		u := resources.Upbound{Unstructured: *uu}

		if event.Type == watch.Modified {
			if resource.IsConditionTrue(u.GetCondition(xpv1.TypeReady)) {
				crdWatcher.Stop()
			}
		}
	}

	return nil
}

func watchDeployments(ctx context.Context, kclient kubernetes.Interface, cancel, stopped chan bool) error {

	spinnerComponents, _ := checkmarkSuccessSpinner.Start(startingComponentWatchText)

	watcher, err := kclient.
		AppsV1().
		Deployments("").
		Watch(ctx, metav1.ListOptions{TimeoutSeconds: &watcherTimeout})
	if err != nil {
		return err
	}

	for {
		event, ok := <-watcher.ResultChan()
		if !ok {
			break
		}

		o, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			continue
		}
		d := resources.Deployment{Deployment: *o}

		text := fmt.Sprintf(infoTextFmt, d.Namespace, d.Name)

		select {
		case <-cancel:
			watcher.Stop()
		default:
			switch event.Type { // nolint: exhaustive
			// we're only interested in adds/updates at this point
			case watch.Added, watch.Modified:
				spinnerComponents.UpdateText(componentText.Sprint(text))
			}
		}
	}

	spinnerComponents.UpdateText(startingComponentWatchText)
	spinnerComponents.Success()
	// inform the shared channel that components are no longer being watched
	stopped <- true

	return nil
}
