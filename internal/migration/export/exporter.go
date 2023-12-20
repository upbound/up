package export

import (
	"context"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XPStateExporter struct {
	kubeCRDs apiextensionsclientset.Interface
}

func NewXPStateExporter() *XPStateExporter {
	return &XPStateExporter{}
}

func (e *XPStateExporter) Export(ctx context.Context) error {

	// List all CRDs
	e.kubeCRDs.ApiextensionsV1().CustomResourceDefinitions().List(ctx context.Context(), v1.ListOptions{})


	return nil
}
