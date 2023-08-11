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

package helm

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	xppkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	xppkgv1alpha1 "github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/upbound/up/internal/resources"
)

var (
	providerName = "provider-helm"
	// Package version to be installed
	version   = "v0.14.0"
	pkgRef, _ = name.ParseReference(fmt.Sprintf("xpkg.upbound.io/crossplane-contrib/provider-helm:%s", version))

	objectsCRD = "releases.helm.crossplane.io"
	xrdCRD     = "compositeresourcedefinitions.apiextensions.crossplane.io"

	pkgGVR = schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "providers",
	}

	ns = "upbound-system"

	ccName  = "provider-helm-hub"
	pkgName = "crossplane-contrib-provider-helm"

	errFmtCreateK8sClient = "failed to create kubernetes client for requirement %s"
	errFmtUXPRequired     = "UXP is required to install %s"
)

// Helm represents provider-helm manager.
type Helm struct {
	crdclient *apixv1client.ApiextensionsV1Client
	dClient   dynamic.Interface
	kclient   kubernetes.Interface
}

func init() {
	// NOTE(tnthornton) we override the runtime.ErrorHandlers so that Helm
	// doesn't leak Println logs.
	runtime.ErrorHandlers = []func(error){} //nolint:reassign
	// NOTE(tnthornton) this suppresses the warnings coming from client-go for
	// using ControllerConfig.
	rest.SetDefaultWarningHandler(rest.NoWarnings{})
}

// New constructs a new CertManager instance that can used to install the
// cert-manager chart.
func New(config *rest.Config) (*Helm, error) {
	crdclient, err := apixv1client.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, providerName))
	}
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, providerName))
	}
	dclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf(errFmtCreateK8sClient, providerName))
	}

	return &Helm{
		crdclient: crdclient,
		dClient:   dclient,
		kclient:   kclient,
	}, nil
}

// GetName returns the name of the provider-helm provider.
func (h *Helm) GetName() string {
	return providerName
}

// Install performs a kubectl apply of the package.
func (h *Helm) Install() error { //nolint:gocyclo
	if h.IsInstalled() {
		// nothing to do
		return nil
	}

	if !h.isUXPInstalled() {
		return fmt.Errorf(errFmtUXPRequired, providerName)
	}

	if err := h.createServiceAccount(); err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return err
		}
	}
	if err := h.createClusterRoleBinding(); err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return err
		}
	}
	if err := h.createControllerConfig(); err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return err
		}
	}

	p := &resources.Package{}
	p.SetName(pkgName)
	p.SetPackage(pkgRef.String())
	p.SetGroupVersionKind(xppkgv1.ProviderGroupVersionKind)
	p.SetControllerConfigRef(xppkgv1.ControllerConfigReference{
		Name: ccName,
	})

	_, err := h.dClient.
		Resource(pkgGVR).
		Create(
			context.Background(),
			p.GetUnstructured(),
			metav1.CreateOptions{},
		)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		p, err := h.dClient.Resource(pkgGVR).Get(ctx, pkgName, metav1.GetOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		pkg := resources.Package{Unstructured: *p}
		if pkg.GetInstalled() && pkg.GetHealthy() {
			break
		}
	}

	for {
		if _, err := h.crdclient.
			CustomResourceDefinitions().
			Get(
				ctx,
				"providerconfigs.helm.crossplane.io",
				metav1.GetOptions{},
			); err == nil {
			break
		}
	}

	return h.createProviderConfig()
}

// IsInstalled checks if provider-helm has been installed in the target cluster.
func (h *Helm) IsInstalled() bool {
	_, err := h.crdclient.
		CustomResourceDefinitions().
		Get(
			context.Background(),
			objectsCRD,
			metav1.GetOptions{},
		)
	return !kerrors.IsNotFound(err)
}

// isUXPInstalled checks if UXP exists in the target cluster.
func (h *Helm) isUXPInstalled() bool {
	_, err := h.crdclient.
		CustomResourceDefinitions().
		Get(
			context.Background(),
			xrdCRD,
			metav1.GetOptions{},
		)
	return !kerrors.IsNotFound(err)
}

func (h *Helm) createServiceAccount() error {
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ccName,
			Namespace: ns,
		},
	}

	_, err := h.kclient.
		CoreV1().
		ServiceAccounts(ns).
		Create(
			context.Background(),
			sa,
			metav1.CreateOptions{},
		)
	return err
}

func (h *Helm) createClusterRoleBinding() error {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: ccName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ccName,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	_, err := h.kclient.
		RbacV1().
		ClusterRoleBindings().
		Create(
			context.Background(),
			crb,
			metav1.CreateOptions{},
		)
	return err
}

func (h *Helm) createControllerConfig() error {
	cc := &resources.ControllerConfig{}
	cc.SetName(ccName)
	cc.SetServiceAccountName(ccName)
	cc.SetGroupVersionKind(xppkgv1alpha1.ControllerConfigGroupVersionKind)

	_, err := h.dClient.
		Resource(resources.ControllerConfigGRV).
		Create(
			context.Background(),
			cc.GetUnstructured(),
			metav1.CreateOptions{},
		)
	return err
}

func (h *Helm) createProviderConfig() error {
	pc := &resources.ProviderConfig{}
	pc.SetName("upbound-cluster")
	pc.SetGroupVersionKind(resources.ProviderConfigHelmGVK)
	pc.SetCredentialsSource(xpv1.CredentialsSourceInjectedIdentity)

	_, err := h.dClient.
		Resource(resources.ProviderConfigHelmGVK.GroupVersion().WithResource("providerconfigs")).
		Create(
			context.Background(),
			pc.GetUnstructured(),
			metav1.CreateOptions{},
		)
	return err
}
