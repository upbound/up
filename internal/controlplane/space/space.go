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

package space

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	xpcommonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/resources"
)

var (
	resource      = resources.ControlPlaneGVK.GroupVersion().WithResource("controlplanes")
	kubeconfigFmt = "kubeconfig-%s"
)

// Client is the client used for interacting with the ControlPlanes API in an
// Upbound Space.
type Client struct {
	c dynamic.Interface
}

// New instantiates a new Client.
func New(c dynamic.Interface) *Client {
	return &Client{
		c: c,
	}
}

// Get the ControlPlane corresponding to the given ControlPlane name.
func (c *Client) Get(ctx context.Context, ctp types.NamespacedName) (*controlplane.Response, error) {
	u, err := c.c.
		Resource(resource).
		Namespace(ctp.Namespace).
		Get(
			ctx,
			ctp.Name,
			metav1.GetOptions{},
		)
	if kerrors.IsNotFound(err) {
		return nil, controlplane.NewNotFound(err)
	}

	if err != nil {
		return nil, err
	}

	return convert(&resources.ControlPlane{Unstructured: *u}), nil
}

// List all ControlPlanes within the Space.
func (c *Client) List(ctx context.Context, namespace string) ([]*controlplane.Response, error) {
	list, err := c.c.
		Resource(resource).
		Namespace(namespace).
		List(
			ctx,
			metav1.ListOptions{},
		)

	if kerrors.IsNotFound(err) {
		return nil, controlplane.NewNotFound(err)
	}

	if err != nil {
		return nil, err
	}

	resps := make([]*controlplane.Response, 0, len(list.Items))
	for _, u := range list.Items {
		resps = append(resps, convert(&resources.ControlPlane{Unstructured: u}))
	}

	return resps, nil
}

// Create a new ControlPlane with the given name and the supplied Options.
func (c *Client) Create(ctx context.Context, name types.NamespacedName, opts controlplane.Options) (*controlplane.Response, error) {
	o := calculateSecret(name.Name, opts)

	ctp := &resources.ControlPlane{}
	ctp.SetName(name.Name)
	ctp.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
		Name:      o.SecretName,
		Namespace: o.SecretNamespace,
	})

	u, err := c.c.
		Resource(resource).
		Namespace(name.Namespace).
		Create(
			ctx,
			ctp.GetUnstructured(),
			metav1.CreateOptions{},
		)
	if err != nil {
		return nil, err
	}

	return convert(&resources.ControlPlane{Unstructured: *u}), nil
}

// Delete the ControlPlane corresponding to the given ControlPlane name.
func (c *Client) Delete(ctx context.Context, ctp types.NamespacedName) error {
	err := c.c.
		Resource(resource).
		Namespace(ctp.Namespace).
		Delete(
			ctx,
			ctp.Name,
			metav1.DeleteOptions{},
		)
	if kerrors.IsNotFound(err) {
		return controlplane.NewNotFound(err)
	}

	return err
}

// GetKubeConfig for the given Control Plane.
func (c *Client) GetKubeConfig(ctx context.Context, ctp types.NamespacedName) (*api.Config, error) {
	// get the control plane
	r, err := c.Get(ctx, ctp)
	if err != nil {
		return nil, err
	}

	// get the corresponding kubeconfig secret
	u, err := c.c.
		Resource(schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "secrets",
		}).
		Namespace(ctp.Name).
		Get(
			ctx,
			r.ConnName,
			metav1.GetOptions{},
		)
	if kerrors.IsNotFound(err) {
		return nil, controlplane.NewNotFound(err)
	}

	// marshal into secret
	var s corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &s); err != nil {
		return nil, err
	}

	return clientcmd.Load(s.Data["kubeconfig"])
}

func convert(ctp *resources.ControlPlane) *controlplane.Response {
	cnd := ctp.GetCondition(xpcommonv1.TypeReady)
	ref := ctp.GetConnectionSecretToReference()
	if ref == nil {
		ref = &xpcommonv1.SecretReference{}
	}

	return &controlplane.Response{
		ID:       ctp.GetControlPlaneID(),
		Name:     ctp.GetName(),
		Group:    ctp.GetNamespace(),
		Message:  cnd.Message,
		Status:   string(cnd.Reason),
		ConnName: ref.Name,
	}
}

func calculateSecret(name string, opts controlplane.Options) controlplane.Options {
	if opts.SecretName == "" {
		opts.SecretName = fmt.Sprintf(kubeconfigFmt, name)
	}
	return opts
}
