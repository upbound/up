// Copyright 2021 Upbound Inc
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

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInputYes(t *testing.T) {

	type args struct {
		input string
	}

	type want struct {
		output bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"InputYes": {
			reason: "We should see true for 'Yes'",
			args: args{
				input: "Yes",
			},
			want: want{
				output: true,
			},
		},
		"InputNo": {
			reason: "We should see false for 'No'",
			args: args{
				input: "No",
			},
			want: want{
				output: false,
			},
		},
		"InputUmm": {
			reason: "We should see false for 'umm'",
			args: args{
				input: "umm",
			},
			want: want{
				output: false,
			},
		},
		"InputEmpty": {
			reason: "We should see false for ''",
			args: args{
				input: "",
			},
			want: want{
				output: false,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			y := inputYes(tc.args.input)

			if diff := cmp.Diff(tc.want.output, y); diff != "" {
				t.Errorf("\n%s\nInputYes(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
