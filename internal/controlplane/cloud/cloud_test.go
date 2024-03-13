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

package cloud

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	sdkerrs "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/common"
	"github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/controlplane"
)

var (
	acct = "demo"

	sdkNotFound = &sdkerrs.Error{
		Status: http.StatusNotFound,
		Detail: ptr.To(`control plane "ctp-dne" not found`),
		Title:  http.StatusText(http.StatusNotFound),
	}

	ctp1 = controlplanes.ControlPlane{
		Name: "ctp1",
		ID:   uuid.MustParse("00000000-0000-0000-0000-000000000000"),
		Configuration: &controlplanes.ControlPlaneConfiguration{
			Name:   ptr.To("cfg1"),
			Status: controlplanes.ConfigurationReady,
		},
	}

	ctp2 = controlplanes.ControlPlane{
		Name: "ctp2",
		ID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Configuration: &controlplanes.ControlPlaneConfiguration{
			Name:   ptr.To("cfg1"),
			Status: controlplanes.ConfigurationReady,
		},
	}

	ctp1Resp = &controlplane.Response{
		Name:    "ctp1",
		ID:      "00000000-0000-0000-0000-000000000000",
		Cfg:     "cfg1",
		Updated: "True",
		Synced:  "True",
		Ready:   "False",
	}

	ctp2Resp = &controlplane.Response{
		Name:    "ctp2",
		ID:      "00000000-0000-0000-0000-000000000001",
		Cfg:     "cfg1",
		Updated: "True",
		Synced:  "True",
		Ready:   "False",
	}
)

type mockCTPClient struct {
	CreateFn func(ctx context.Context, account string, params *controlplanes.ControlPlaneCreateParameters) (*controlplanes.ControlPlaneResponse, error)
	DeleteFn func(ctx context.Context, account, name string) error
	GetFn    func(ctx context.Context, account, name string) (*controlplanes.ControlPlaneResponse, error)
	ListFn   func(ctx context.Context, account string, opts ...common.ListOption) (*controlplanes.ControlPlaneListResponse, error)
}

func (m *mockCTPClient) Create(ctx context.Context, account string, params *controlplanes.ControlPlaneCreateParameters) (*controlplanes.ControlPlaneResponse, error) {
	return m.CreateFn(ctx, account, params)
}

func (m *mockCTPClient) Delete(ctx context.Context, account, name string) error {
	return m.DeleteFn(ctx, account, name)
}

func (m *mockCTPClient) Get(ctx context.Context, account, name string) (*controlplanes.ControlPlaneResponse, error) {
	return m.GetFn(ctx, account, name)
}

func (m *mockCTPClient) List(ctx context.Context, account string, opts ...common.ListOption) (*controlplanes.ControlPlaneListResponse, error) {
	return m.ListFn(ctx, account, opts...)
}

func TestGet(t *testing.T) {
	type args struct {
		ctp  ctpClient
		cfg  cfgGetter
		name string
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
				ctp: &mockCTPClient{
					GetFn: func(ctx context.Context, account, name string) (*controlplanes.ControlPlaneResponse, error) {
						return nil, sdkNotFound
					},
				},
				name: "ctp-dne",
			},
			want: want{
				err: controlplane.NewNotFound(errors.New(`Not Found: control plane "ctp-dne" not found`)),
			},
		},
		"Success": {
			reason: "If the control plane exists, a response is returned.",
			args: args{
				ctp: &mockCTPClient{
					GetFn: func(ctx context.Context, account, name string) (*controlplanes.ControlPlaneResponse, error) {
						return &controlplanes.ControlPlaneResponse{
							ControlPlane: ctp1,
						}, nil
					},
				},
				name: "ctp1",
			},
			want: want{
				resp: ctp1Resp,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := New(tc.args.ctp, tc.args.cfg, acct)
			got, err := c.Get(context.Background(), types.NamespacedName{Name: tc.args.name})

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
	type args struct {
		ctp  ctpClient
		cfg  cfgGetter
		name string
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
				ctp: &mockCTPClient{
					DeleteFn: func(ctx context.Context, account, name string) error {
						return sdkNotFound
					},
				},
				name: "ctp-dne",
			},
			want: want{
				err: controlplane.NewNotFound(errors.New(`Not Found: control plane "ctp-dne" not found`)),
			},
		},
		"Success": {
			reason: "If the control plane exists, no error is returned.",
			args: args{
				ctp: &mockCTPClient{
					DeleteFn: func(ctx context.Context, account, name string) error {
						return nil
					},
				},
				name: "ctp1",
			},
			want: want{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := New(tc.args.ctp, tc.args.cfg, acct)
			err := c.Delete(context.Background(), types.NamespacedName{Name: tc.args.name})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDelete(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestList(t *testing.T) {
	type args struct {
		ctp ctpClient
		cfg cfgGetter
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
				ctp: &mockCTPClient{
					ListFn: func(ctx context.Context, account string, opts ...common.ListOption) (*controlplanes.ControlPlaneListResponse, error) {
						return &controlplanes.ControlPlaneListResponse{
							ControlPlanes: []controlplanes.ControlPlaneResponse{},
						}, nil
					},
				},
			},
			want: want{
				resp: []*controlplane.Response{},
			},
		},
		"SingleControlPlane": {
			reason: "If a single control plane exists, a response with only the one control plane is returned.",
			args: args{
				ctp: &mockCTPClient{
					ListFn: func(ctx context.Context, account string, opts ...common.ListOption) (*controlplanes.ControlPlaneListResponse, error) {
						return &controlplanes.ControlPlaneListResponse{
							ControlPlanes: []controlplanes.ControlPlaneResponse{
								{
									ControlPlane: ctp1,
								},
							},
						}, nil
					},
				},
			},
			want: want{
				resp: []*controlplane.Response{
					ctp1Resp,
				},
			},
		},
		"MultiControlPlanes": {
			reason: "If multiple control plane exists, a response with each of the control planes is returned.",
			args: args{
				ctp: &mockCTPClient{
					ListFn: func(ctx context.Context, account string, opts ...common.ListOption) (*controlplanes.ControlPlaneListResponse, error) {
						return &controlplanes.ControlPlaneListResponse{
							ControlPlanes: []controlplanes.ControlPlaneResponse{
								{
									ControlPlane: ctp1,
								},
								{
									ControlPlane: ctp2,
								},
							},
						}, nil
					},
				},
			},
			want: want{
				resp: []*controlplane.Response{
					ctp1Resp,
					ctp2Resp,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := New(tc.args.ctp, tc.args.cfg, acct)
			got, err := c.List(context.Background(), "")

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nList(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.resp, got); diff != "" {
				t.Errorf("\n%s\nList(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	type args struct {
		ctp *controlplanes.ControlPlaneResponse
	}
	type want struct {
		resp *controlplane.Response
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ConfigurationAssociated": {
			reason: "If a configuration is associated with the control plane, return response with name and status.",
			args: args{
				ctp: &controlplanes.ControlPlaneResponse{
					ControlPlane: ctp1,
				},
			},
			want: want{
				resp: ctp1Resp,
			},
		},
		"ConfigurationNotAssociated": {
			reason: "If a configuration is not associated with the control plane, response has n/a for configuration name and status.",
			args: args{
				ctp: &controlplanes.ControlPlaneResponse{
					ControlPlane: controlplanes.ControlPlane{
						Name: "ctp1",
						ID:   uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					},
				},
			},
			want: want{
				resp: &controlplane.Response{
					Name:    "ctp1",
					ID:      "00000000-0000-0000-0000-000000000000",
					Synced:  "True",
					Ready:   "False",
					Cfg:     "",
					Updated: "",
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
