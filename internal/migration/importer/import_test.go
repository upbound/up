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

package importer

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/google/go-cmp/cmp"
)

func Test_printConditions(t *testing.T) {
	type args struct {
		conditions []xpv1.ConditionType
	}
	type want struct {
		out string
	}
	cases := map[string]struct {
		args args
		want want
	}{
		"Empty": {
			args: args{
				conditions: []xpv1.ConditionType{},
			},
			want: want{
				out: "",
			},
		},
		"Single": {
			args: args{
				conditions: []xpv1.ConditionType{
					xpv1.TypeReady,
				},
			},
			want: want{
				out: "Ready",
			},
		},
		"Double": {
			args: args{
				conditions: []xpv1.ConditionType{
					xpv1.TypeReady,
					xpv1.TypeSynced,
				},
			},
			want: want{
				out: "Ready and Synced",
			},
		},
		"More": {
			args: args{
				conditions: []xpv1.ConditionType{
					xpv1.TypeReady,
					xpv1.TypeSynced,
					"Installed",
					"Healthy",
				},
			},
			want: want{
				out: "Ready, Synced, Installed, and Healthy",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := printConditions(tc.args.conditions)
			if diff := cmp.Diff(got, tc.want.out); diff != "" {
				t.Errorf("printConditions() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
