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

package billing

import (
	"fmt"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	usagetime "github.com/upbound/up/internal/usage/time"
)

func TestGetBillingPeriod(t *testing.T) {
	type args struct {
		billingMonth  time.Time
		billingCustom *dateRange
	}
	type want struct {
		billingPeriod usagetime.Range
		err           error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Unset": {
			reason: "Setting no billing period should return an error.",
			args:   args{},
			want: want{
				err: fmt.Errorf("billing period is not set"),
			},
		},
		"BillingMonth": {
			reason: "A billing month should cover the entire month.",
			args: args{
				billingMonth: time.Date(2006, 5, 1, 0, 0, 0, 0, time.UTC),
			},
			want: want{
				billingPeriod: usagetime.Range{
					Start: time.Date(2006, 5, 1, 0, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 6, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		"BillingCustom": {
			reason: "A custom billing period should cover the start of the start date to the end of the end date.",
			args: args{
				billingCustom: &dateRange{
					Start: time.Date(2006, 5, 4, 0, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 7, 0, 0, 0, 0, time.UTC),
				},
			},
			want: want{
				billingPeriod: usagetime.Range{
					Start: time.Date(2006, 5, 4, 0, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 8, 0, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &exportCmd{
				BillingMonth:  tc.args.billingMonth,
				BillingCustom: tc.args.billingCustom,
			}

			got, err := c.getBillingPeriod()
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ngetBillingPeriod(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.billingPeriod, got); diff != "" {
				t.Errorf("\n%s\ngetBillingPeriod(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
