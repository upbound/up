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

package ingressnginx

import (
	"context"
	"fmt"
	"net/url"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
)

var (
	chartName   = "ingress-nginx"
	nginxURL, _ = url.Parse("https://kubernetes.github.io/ingress-nginx")

	// Chart version to be installed
	version = "4.7.1"
	// Ensure we don't request a LoadBalancer to be deployed.
	// xref: https://github.com/kubernetes/ingress-nginx/blob/main/hack/manifest-templates/provider/kind/values.yaml
	values = map[string]any{
		"controller": map[string]any{
			"updateStrategy": map[string]any{
				"type": "RollingUpdate",
				"rollingUpdate": map[string]any{
					"maxUnavailable": 1,
				},
			},
			"hostPort": map[string]any{
				"enabled": true,
			},
			"terminationGracePeriodSeconds": 0,
			"service": map[string]any{
				"type": "NodePort",
			},
			"watchIngressWithoutClass": true,
			"nodeSelector": map[string]any{
				"ingress-ready": "true",
			},
			"tolerations": []map[string]string{
				{
					"key":      "node-role.kubernetes.io/master",
					"operator": "Equal",
					"effect":   "NoSchedule",
				},
				{
					"key":      "node-role.kubernetes.io/control-plane",
					"operator": "Equal",
					"effect":   "NoSchedule",
				},
			},
			"publishService": map[string]any{
				"enabled": false,
			},
			"extraArgs": map[string]any{
				"publish-status-address": "localhost",
			},
		},
	}

	errFmtCreateHelmManager = "failed to create helm manager for %s"
	errFmtCreateK8sClient   = "failed to create kubernetes client for helm chart %s"
	errFmtCreateNamespace   = "failed to create namespace %s"
)

// IngressNginx represents a Helm manager
type IngressNginx struct {
	mgr     install.Manager
	kclient kubernetes.Interface
	dclient dynamic.Interface
}

// New constructs a new CertManager instance that can used to install the
// cert-manager chart.
func New(config *rest.Config) (*IngressNginx, error) {
	mgr, err := helm.NewManager(config,
		chartName,
		nginxURL,
		helm.WithNamespace(chartName),
	)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateHelmManager, chartName))
	}
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}
	dclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, chartName))
	}

	return &IngressNginx{
		mgr:     mgr,
		dclient: dclient,
		kclient: kclient,
	}, nil
}

// GetName returns the name of the cert-manager chart.
func (c *IngressNginx) GetName() string {
	return chartName
}

// Install performs a Helm install of the chart.
func (c *IngressNginx) Install() error { //nolint:gocyclo
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

	if err := c.mgr.Install(version, values); err != nil {
		return err
	}

	for {
		d, err := c.kclient.
			AppsV1().
			Deployments(chartName).
			Get(
				context.Background(),
				"ingress-nginx-controller",
				metav1.GetOptions{},
			)
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		if d.Status.Replicas == d.Status.ReadyReplicas {
			// deployment is ready
			break
		}
	}

	return nil
}

// IsInstalled checks if cert-manager has been installed in the target cluster.
func (c *IngressNginx) IsInstalled() bool {
	il, err := c.kclient.
		NetworkingV1().
		IngressClasses().
		List(
			context.Background(),
			metav1.ListOptions{},
		)

	// Separate check in the event il comes back nil.
	if il != nil && len(il.Items) == 0 {
		return false
	}

	return !kerrors.IsNotFound(err)
}
