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

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestUpdateKubeConfig(t *testing.T) {
	type args struct {
		cfg     api.Config
		account string
		ctpName string
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
		"ErrorMissingCluster": {
			reason: "If the supplied config is missing the expected cluster, an error is returned.",
			args: args{
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"demo-ctp1": {},
					},
					Contexts: map[string]*api.Context{
						"demo-ctp1": {
							Cluster:  "demo-ctp1",
							AuthInfo: "demo-ctp1",
						},
					},
					CurrentContext: "demo-ctp1",
				},
				account: "demo",
				ctpName: "ctp1",
				context: "kind-kind",
			},
			want: want{
				cfg: api.Config{},
				err: errors.New(`config is broken, missing cluster: "demo-ctp1"`),
			},
		},
		"ErrorMissingUser": {
			reason: "If the supplied config is missing the expected user, an error is returned.",
			args: args{
				cfg: api.Config{
					Clusters: map[string]*api.Cluster{
						"demo-ctp1": {},
					},
					Contexts: map[string]*api.Context{
						"demo-ctp1": {
							Cluster:  "demo-ctp1",
							AuthInfo: "demo-ctp1",
						},
					},
					CurrentContext: "demo-ctp1",
				},
				account: "demo",
				ctpName: "ctp1",
				context: "kind-kind",
			},
			want: want{
				cfg: api.Config{},
				err: errors.New(`config is broken, missing user: "demo-ctp1"`),
			},
		},
		"ErrorMissingContext": {
			reason: "If the supplied config is missing the expected context, an error is returned.",
			args: args{
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"demo-ctp1": {},
					},
					Clusters: map[string]*api.Cluster{
						"demo-ctp1": {},
					},
					CurrentContext: "demo-ctp1",
				},
				account: "demo",
				ctpName: "ctp1",
				context: "kind-kind",
			},
			want: want{
				cfg: api.Config{},
				err: errors.New(`config is broken, missing context: "demo-ctp1"`),
			},
		},
		"Success": {
			reason: "Supplying an account, control plane name, and context should update the config.",
			args: args{
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"demo-ctp1": {},
					},
					Clusters: map[string]*api.Cluster{
						"demo-ctp1": {},
					},
					Contexts: map[string]*api.Context{
						"demo-ctp1": {
							Cluster:  "demo-ctp1",
							AuthInfo: "demo-ctp1",
						},
					},
					CurrentContext: "demo-ctp1",
				},
				account: "demo",
				ctpName: "ctp1",
				context: "kind-kind",
			},
			want: want{
				cfg: api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"upbound_demo_ctp1_kind-kind": {},
					},
					Clusters: map[string]*api.Cluster{
						"upbound_demo_ctp1_kind-kind": {},
					},
					Contexts: map[string]*api.Context{
						"upbound_demo_ctp1_kind-kind": {
							Cluster:  "upbound_demo_ctp1_kind-kind",
							AuthInfo: "upbound_demo_ctp1_kind-kind",
						},
					},
					CurrentContext: "upbound_demo_ctp1_kind-kind",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got, err := updateKubeConfig(tc.args.cfg, tc.args.account, tc.args.ctpName, tc.args.context)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpdateKubeConfig(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cfg, got); diff != "" {
				t.Errorf("\n%s\nUpdateKubeConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
