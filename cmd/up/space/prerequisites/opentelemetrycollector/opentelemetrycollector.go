// Copyright 2024 Upbound Inc
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

package opentelemetrycollector

import (
	"context"
	"fmt"
	"net/url"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

var (
	chartName     = "opentelemetry-operator"
	otelMgrURL, _ = url.Parse("https://open-telemetry.github.io/opentelemetry-helm-charts")

	// Chart version to be installed
	version = "0.56.0"

	// Set image used to contrib to cover more exporters
	values = map[string]any{
		"manager": map[string]any{
			"collectorImage": map[string]any{
				"repository": "otel/opentelemetry-collector-contrib",
			},
		},
	}

	otelCollectorCRD = "opentelemetrycollector.opentelemetry.io"

	errFmtCreateHelmManager = "failed to create helm manager for %s"
	errFmtCreateK8sClient   = "failed to create kubernetes client for helm chart %s"
	errFmtCreateNamespace   = "failed to create namespace %s"
)

// OpenTelemetryCollectorOperator represents a Helm manager
type OpenTelemetryCollectorOperator struct {
	mgr       install.Manager
	crdclient *apixv1client.ApiextensionsV1Client
	kclient   kubernetes.Interface
}

// New constructs a new OpenTelemetryCollectorMgr instance that can used to install the
// opentelemetry-operator chart.
func New(config *rest.Config) (*OpenTelemetryCollectorOperator, error) {
	mgr, err := helm.NewManager(config,
		chartName,
		otelMgrURL,
		helm.WithNamespace(chartName),
	)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateHelmManager, chartName))
	}
	crdclient, err := apixv1client.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}

	return &OpenTelemetryCollectorOperator{
		mgr:       mgr,
		crdclient: crdclient,
		kclient:   kclient,
	}, nil
}

// GetName returns the name of the opentelemetry-operator chart.
func (c *OpenTelemetryCollectorOperator) GetName() string {
	return chartName
}

// Install performs a Helm install of the chart.
func (c *OpenTelemetryCollectorOperator) Install() error {
	if c.IsInstalled() {
		// nothing to do
		return nil
	}

	// create namespace before creating chart.
	_, err := c.kclient.CoreV1().
		Namespaces().
		Create(context.Background(),
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: chartName,
				},
			}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, fmt.Sprintf(errFmtCreateNamespace, chartName))
	}

	return c.mgr.Install(version, values)
}

// IsInstalled checks if opentelemetry operator has been installed in the target cluster.
func (c *OpenTelemetryCollectorOperator) IsInstalled() bool {
	_, err := c.crdclient.
		CustomResourceDefinitions().
		Get(
			context.Background(),
			otelCollectorCRD,
			metav1.GetOptions{},
		)
	return !kerrors.IsNotFound(err)
}
