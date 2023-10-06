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
	"fmt"
	"reflect"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

func TestInitVersionConstraints(t *testing.T) {
	for _, vc := range initVersionConstraints {
		_, err := semver.NewConstraint(vc.semver)
		if err != nil {
			t.Errorf("failed to parse constraint %q: %v", vc.semver, err)
		}
	}
}

func TestUpgradeVersionConstraints(t *testing.T) {
	for _, vc := range upgradeVersionConstraints {
		_, err := semver.NewConstraint(vc.semver)
		if err != nil {
			t.Errorf("failed to parse constraint %q: %v", vc.semver, err)
		}
	}
}

func TestUpgradeFromVersionConstraints(t *testing.T) {
	for _, vc := range upgradeFromVersionConstraints {
		_, err := semver.NewConstraint(vc.semver)
		if err != nil {
			t.Errorf("failed to parse constraint %q: %v", vc.semver, err)
		}
	}
}

func Test_parseChartUpConstraints(t *testing.T) {
	type want struct {
		constraints []constraint
		err         bool
	}
	tests := []struct {
		name string
		arg  string
		want want
	}{
		{
			name: "empty",
			arg:  "",
			want: want{
				constraints: nil,
			},
		},
		{
			name: "one",
			arg:  ">= 1.10: up 1.10.0 or later is required",
			want: want{
				constraints: []constraint{{
					semver:  ">= 1.10",
					message: "up 1.10.0 or later is required",
				}},
			},
		},
		{
			name: "multiple",
			arg:  ">= 1.10: up 1.10.0 or later is required;< 1.20: up <1.20 is required",
			want: want{
				constraints: []constraint{{
					semver:  ">= 1.10",
					message: "up 1.10.0 or later is required",
				}, {
					semver:  "< 1.20",
					message: "up <1.20 is required",
				}},
			},
		},
		{
			name: "invalid",
			arg:  ">= 1.10: up 1.10.0 or later is required;invalid",
			want: want{
				err: true,
			},
		},
		{
			name: "colon",
			arg:  ">= 1.10: up 1.10.0 or later is required: because reason",
			want: want{
				constraints: []constraint{{
					semver:  ">= 1.10",
					message: "up 1.10.0 or later is required: because reason",
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChartUpConstraints(tt.arg)
			if (err != nil) != tt.want.err {
				t.Errorf("parseChartUpConstraints() error = %v, want.err %v", err, tt.want.err)
				return
			}
			if !reflect.DeepEqual(got, tt.want.constraints) {
				t.Errorf("parseChartUpConstraints() got = %v, want.constraints %v", got, tt.want)
			}
		})
	}
}

func TestCheckVersion(t *testing.T) {
	errBoom := "boom"

	type args struct {
		constraints []constraint
		version     string
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrorVersionGreaterThanBoundary": {
			reason: "If the supplied version fails the check an error is returned.",
			args: args{
				version:     "1.0",
				constraints: []constraint{{semver: "<1.0", message: "versions must be less than 1.0"}},
			},
			want: want{
				err: fmt.Errorf("%s: versions must be less than 1.0", errBoom),
			},
		},
		"ErrorVersionLessThanBoundary": {
			reason: "If the supplied version fails the check an error is returned.",
			args: args{
				version:     "0.10",
				constraints: []constraint{{semver: ">=1.0", message: "versions must be greater than 1.0"}},
			},
			want: want{
				err: fmt.Errorf("%s: versions must be greater than 1.0", errBoom),
			},
		},
		"SuccessInitStandardVersion": {
			reason: "A standard version that falls within our initVersionConstraints should not return an error.",
			args: args{
				version:     "1.0",
				constraints: initVersionConstraints,
			},
		},
		"SuccessInitPreReleaseVersion": {
			reason: "A pre-release version that falls within our initVersionConstraints should not return an error.",
			args: args{
				version:     "1.0.0-rc.1",
				constraints: initVersionConstraints,
			},
		},
		"SuccessUpgradeStandardVersion": {
			reason: "A standard version that falls within our upgradeVersionConstraints should not return an error.",
			args: args{
				version:     "1.0",
				constraints: upgradeVersionConstraints,
			},
		},
		"SuccessUpgradePreReleaseVersion": {
			reason: "A pre-release version that falls within our upgradeVersionConstraints should not return an error.",
			args: args{
				version:     "1.0.0-rc.1",
				constraints: upgradeVersionConstraints,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := checkVersion(errBoom, tc.args.constraints, tc.args.version)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ncheckVersion(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
