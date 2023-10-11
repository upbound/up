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

package controlplane

import (
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestOrigContext(t *testing.T) {
	type args struct {
		context string
	}
	type want struct {
		result string
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorInvalidContextName": {
			reason: "We expect that there are 4 parts in a well formed context.",
			args: args{
				context: "demo-ctp1",
			},
			want: want{
				err: errors.New("given context does not have the correct number of parts, expected: 4, got: 1"),
			},
		},
		"Success": {
			reason: "A well formed context name should be parsed successfully.",
			args: args{
				context: "upbound_demo_cpt1_kind-kind",
			},
			want: want{
				result: "kind-kind",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got, err := origContext(tc.args.context)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nOrigContext(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nOrigContext(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRemoveFromConfig(t *testing.T) {
	type args struct {
		cfg     api.Config
		context string
	}
	type want struct {
		cfg api.Config
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorCurrentContext": {
			reason: "If the current context is equal to the provided context an error is returned.",
			args: args{
				context: "upbound_demo_ctp1_kind-kind",
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Clusters: map[string]*api.Cluster{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Contexts: map[string]*api.Context{
						"upbound_demo_ctp1_kind-kind": {
							Cluster:  "upbound_demo_ctp1_kind-kind",
							AuthInfo: "upbound_demo_ctp1_kind-kind",
						},
						"kind-kind": {
							Cluster:  "kind-kind",
							AuthInfo: "kind-kind",
						},
					},
					CurrentContext: "upbound_demo_ctp1_kind-kind",
				},
			},
			want: want{
				err: errors.New(`context "upbound_demo_ctp1_kind-kind" is currently in use`),
			},
		},
		"InvalidContextName": {
			reason: "No change to the config occurs if we're given a context that does not exist.",
			args: args{
				context: "upbound_demo_dne_kind-kind",
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Clusters: map[string]*api.Cluster{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Contexts: map[string]*api.Context{
						"upbound_demo_ctp1_kind-kind": {
							Cluster:  "upbound_demo_ctp1_kind-kind",
							AuthInfo: "upbound_demo_ctp1_kind-kind",
						},
						"kind-kind": {
							Cluster:  "kind-kind",
							AuthInfo: "kind-kind",
						},
					},
					CurrentContext: "kind-kind",
				},
			},
			want: want{
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Clusters: map[string]*api.Cluster{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Contexts: map[string]*api.Context{
						"upbound_demo_ctp1_kind-kind": {
							Cluster:  "upbound_demo_ctp1_kind-kind",
							AuthInfo: "upbound_demo_ctp1_kind-kind",
						},
						"kind-kind": {
							Cluster:  "kind-kind",
							AuthInfo: "kind-kind",
						},
					},
					CurrentContext: "kind-kind",
				},
			},
		},
		"Success": {
			reason: "A well formed context name should be parsed successfully.",
			args: args{
				context: "upbound_demo_ctp1_kind-kind",
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Clusters: map[string]*api.Cluster{
						"upbound_demo_ctp1_kind-kind": {},
						"kind-kind":                   {},
					},
					Contexts: map[string]*api.Context{
						"upbound_demo_ctp1_kind-kind": {
							Cluster:  "upbound_demo_ctp1_kind-kind",
							AuthInfo: "upbound_demo_ctp1_kind-kind",
						},
						"kind-kind": {
							Cluster:  "kind-kind",
							AuthInfo: "kind-kind",
						},
					},
					CurrentContext: "kind-kind",
				},
			},
			want: want{
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"kind-kind": {},
					},
					Clusters: map[string]*api.Cluster{
						"kind-kind": {},
					},
					Contexts: map[string]*api.Context{
						"kind-kind": {
							Cluster:  "kind-kind",
							AuthInfo: "kind-kind",
						},
					},
					CurrentContext: "kind-kind",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got, err := removeFromConfig(tc.args.cfg, tc.args.context)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRemoveFromConfig(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cfg, got); diff != "" {
				t.Errorf("\n%s\nRemoveFromConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
