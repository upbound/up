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

	"github.com/crossplane/crossplane-runtime/pkg/test"
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
		"SuccessNamespaceLess": {
			reason: "A well formed context name should be parsed successfully.",
			args: args{
				context: "upbound_demo_cpt1_kind-kind",
			},
			want: want{
				result: "kind-kind",
			},
		},
		"Success": {
			reason: "A well formed context name should be parsed successfully.",
			args: args{
				context: "upbound_demo_default/cpt1_kind-kind",
			},
			want: want{
				result: "kind-kind",
			},
		},
		"SuccessUnderscoreNamespaceLess": {
			reason: "A well formed context name should be parsed successfully.",
			args: args{
				context: "upbound_demo_cpt1_kind_kind_foo_bar",
			},
			want: want{
				result: "kind_kind_foo_bar",
			},
		},
		"SuccessUnderscore": {
			reason: "A well formed context name should be parsed successfully.",
			args: args{
				context: "upbound_demo_default/cpt1_kind_kind_foo_bar",
			},
			want: want{
				result: "kind_kind_foo_bar",
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
