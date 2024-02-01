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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	cgotesting "k8s.io/client-go/testing"

	xpcommonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/resources"
)

var (
	ctpresource = "controlplanes"

	controlPlaneGRV = schema.GroupResource{
		Group:    "spaces.upbound.io",
		Resource: ctpresource,
	}

	scheme = runtime.NewScheme()
)

func TestGet(t *testing.T) {
	ctp1 := &resources.ControlPlane{}
	ctp1.SetName("ctp1")
	ctp1.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
		Name:      "kubeconfig-ctp1",
		Namespace: "default",
	})

	type args struct {
		client    dynamic.Interface
		name      string
		namespace string
	}
	type want struct {
		resp *controlplane.Response
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorControlPlaneNotFound": {
			reason: "If the requested control plane does not exist, a not found error is returned.",
			args: args{
				client: func() dynamic.Interface {
					c := fake.NewSimpleDynamicClient(scheme)
					c.PrependReactor(
						"get",
						ctpresource,
						func(action cgotesting.Action) (handled bool, ret runtime.Object, err error) {
							return true, nil, kerrors.NewNotFound(controlPlaneGRV, "ctp-dne")
						})

					return c
				}(),
				name: "ctp-dne",
			},
			want: want{
				err: controlplane.NewNotFound(errors.New(`controlplanes.spaces.upbound.io "ctp-dne" not found`)),
			},
		},
		"Success": {
			reason: "If the control plane exists, a response is returned.",
			args: args{
				client: fake.NewSimpleDynamicClient(scheme, ctp1.GetUnstructured()),
				name:   "ctp1",
			},
			want: want{
				resp: &controlplane.Response{
					Name:     "ctp1",
					ConnName: "kubeconfig-ctp1",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := New(tc.args.client)
			got, err := c.Get(context.Background(), types.NamespacedName{Name: tc.args.name, Namespace: tc.args.namespace})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.resp, got); diff != "" {
				t.Errorf("\n%s\nGet(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctp1 := &resources.ControlPlane{}
	ctp1.SetName("ctp1")
	ctp1.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
		Name:      "kubeconfig-ctp1",
		Namespace: "default",
	})

	type args struct {
		client    dynamic.Interface
		name      string
		namespace string
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorControlPlaneNotFound": {
			reason: "If the requested control plane does not exist, a not found error is returned.",
			args: args{
				client: func() dynamic.Interface {
					c := fake.NewSimpleDynamicClient(scheme)
					c.PrependReactor(
						"delete",
						ctpresource,
						func(action cgotesting.Action) (handled bool, ret runtime.Object, err error) {
							return true, nil, kerrors.NewNotFound(controlPlaneGRV, "ctp-dne")
						})

					return c
				}(),
				name: "ctp-dne",
			},
			want: want{
				err: controlplane.NewNotFound(errors.New(`controlplanes.spaces.upbound.io "ctp-dne" not found`)),
			},
		},
		"Success": {
			reason: "If the control plane exists, no error is returned.",
			args: args{
				client: fake.NewSimpleDynamicClient(scheme, ctp1.GetUnstructured()),
				name:   "ctp1",
			},
			want: want{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := New(tc.args.client)
			err := c.Delete(context.Background(), types.NamespacedName{Name: tc.args.name, Namespace: tc.args.namespace})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDelete(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestList(t *testing.T) {
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "spaces.upbound.io",
		Version: "v1beta1",
		Kind:    "ControlPlaneList"},
		&unstructured.UnstructuredList{},
	)

	ctp1 := &resources.ControlPlane{}
	ctp1.SetName("ctp1")
	ctp1.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
		Name:      "kubeconfig-ctp1",
		Namespace: "default",
	})

	ctp2 := &resources.ControlPlane{}
	ctp2.SetName("ctp2")
	ctp2.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
		Name:      "kubeconfig-ctp2",
		Namespace: "default",
	})

	type args struct {
		client    dynamic.Interface
		namespace string
	}
	type want struct {
		resp []*controlplane.Response
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoControlPlanes": {
			reason: "If there are no control planes, an empty response slice is returned.",
			args: args{
				client: fake.NewSimpleDynamicClient(scheme),
			},
			want: want{
				resp: []*controlplane.Response{},
			},
		},
		"SingleControlPlane": {
			reason: "If a single control plane exists, a response with only the one control plane is returned.",
			args: args{
				client: fake.NewSimpleDynamicClient(
					scheme,
					ctp1.GetUnstructured(),
				),
			},
			want: want{
				resp: []*controlplane.Response{
					{
						Name:     "ctp1",
						ConnName: "kubeconfig-ctp1",
					},
				},
			},
		},
		"MultiControlPlanes": {
			reason: "If multiple control plane exists, a response with each of the control planes is returned.",
			args: args{
				client: fake.NewSimpleDynamicClient(
					scheme,
					ctp1.GetUnstructured(),
					ctp2.GetUnstructured(),
				),
			},
			want: want{
				resp: []*controlplane.Response{
					{
						Name:     "ctp1",
						ConnName: "kubeconfig-ctp1",
					},
					{
						Name:     "ctp2",
						ConnName: "kubeconfig-ctp2",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := New(tc.args.client)
			got, err := c.List(context.Background(), tc.args.namespace)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nList(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.resp, got); diff != "" {
				t.Errorf("\n%s\nList(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetKubeConfig(t *testing.T) {
	ctp1 := &resources.ControlPlane{}
	ctp1.SetName("ctp1")
	ctp1.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
		Name:      "kubeconfig-ctp1",
		Namespace: "default",
	})

	type args struct {
		client    dynamic.Interface
		name      string
		namespace string
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorControlPlaneNotFound": {
			reason: "If the requested control plane does not exist, a not found error is returned.",
			args: args{
				client: func() dynamic.Interface {
					c := fake.NewSimpleDynamicClient(scheme)
					c.PrependReactor(
						"delete",
						ctpresource,
						func(action cgotesting.Action) (handled bool, ret runtime.Object, err error) {
							return true, nil, kerrors.NewNotFound(controlPlaneGRV, "ctp-dne")
						})

					return c
				}(),
				name: "ctp-dne",
			},
			want: want{
				err: controlplane.NewNotFound(errors.New(`controlplanes.spaces.upbound.io "ctp-dne" not found`)),
			},
		},
		"Success": {
			reason: "If the control plane exists, no error is returned.",
			args: args{
				client: fake.NewSimpleDynamicClient(scheme, ctp1.GetUnstructured()),
				name:   "ctp1",
			},
			want: want{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := New(tc.args.client)
			err := c.Delete(context.Background(), types.NamespacedName{Name: tc.args.name, Namespace: tc.args.namespace})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDelete(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCalculateSecret(t *testing.T) {
	type args struct {
		name string
		opts controlplane.Options
	}
	type want struct {
		opts controlplane.Options
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"DefaultSecretName": {
			reason: "If secretname is not supplied, the default is calculated.",
			args: args{
				name: "ctp1",
				opts: controlplane.Options{},
			},
			want: want{
				opts: controlplane.Options{
					SecretName: "kubeconfig-ctp1",
				},
			},
		},
		"SuppliedSecretName": {
			reason: "If secretname is supplied, the secretname is preserved.",
			args: args{
				name: "ctp1",
				opts: controlplane.Options{
					SecretName: "supplied",
				},
			},
			want: want{
				opts: controlplane.Options{
					SecretName: "supplied",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got := calculateSecret(tc.args.name, tc.args.opts)

			if diff := cmp.Diff(tc.want.opts, got); diff != "" {
				t.Errorf("\n%s\ncalculateSecret(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	type args struct {
		ctp *resources.ControlPlane
	}
	type want struct {
		resp *controlplane.Response
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ConditionEmptyMessage": {
			reason: "If the ready condition is has a status of true, the message is empty.",
			args: args{
				ctp: func() *resources.ControlPlane {
					c := &resources.ControlPlane{}
					c.SetName("ctp1")
					c.SetControlPlaneID("mxp1")
					c.SetNamespace("default")
					c.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
						Name:      "kubeconfig-ctp1",
						Namespace: "default",
					})
					c.SetConditions([]xpcommonv1.Condition{xpcommonv1.ReconcileSuccess()}...)
					c.SetConditions([]xpcommonv1.Condition{xpcommonv1.Available()}...)
					c.SetAnnotations(map[string]string{"internal.spaces.upbound.io/message": ""})

					return c
				}(),
			},
			want: want{
				resp: &controlplane.Response{
					Name:     "ctp1",
					ID:       "mxp1",
					Group:    "default",
					Synced:   "True",
					Ready:    "True",
					ConnName: "kubeconfig-ctp1",
					Message:  "",
				},
			},
		},
		"ConditionHasMessage": {
			reason: "If the ready condition is has a status of false, the message is not empty.",
			args: args{
				ctp: func() *resources.ControlPlane {
					c := &resources.ControlPlane{}
					c.SetName("ctp1")
					c.SetControlPlaneID("mxp1")
					c.SetNamespace("default")
					c.SetWriteConnectionSecretToReference(&xpcommonv1.SecretReference{
						Name:      "kubeconfig-ctp1",
						Namespace: "default",
					})
					c.SetConditions(xpcommonv1.ReconcileSuccess())
					c.SetConditions(xpcommonv1.Creating().WithMessage("something"))
					c.SetAnnotations(map[string]string{"internal.spaces.upbound.io/message": "creating..."})

					return c
				}(),
			},
			want: want{
				resp: &controlplane.Response{
					Name:     "ctp1",
					ID:       "mxp1",
					Group:    "default",
					Synced:   "True",
					Ready:    "False",
					Message:  "creating...",
					ConnName: "kubeconfig-ctp1",
				},
			},
		},
		"EmptyConnectionSecret": {
			reason: "If the control plane does not have a connection secret set, connection details are empty in the response.",
			args: args{
				ctp: func() *resources.ControlPlane {
					c := &resources.ControlPlane{}
					c.SetName("ctp1")
					c.SetControlPlaneID("mxp1")
					c.SetConditions(xpcommonv1.ReconcileSuccess())
					c.SetConditions([]xpcommonv1.Condition{xpcommonv1.Available()}...)

					return c
				}(),
			},
			want: want{
				resp: &controlplane.Response{
					Name:   "ctp1",
					ID:     "mxp1",
					Synced: "True",
					Ready:  "True",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got := convert(tc.args.ctp)

			if diff := cmp.Diff(tc.want.resp, got); diff != "" {
				t.Errorf("\n%s\nconvert(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
