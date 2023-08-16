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

package azure

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/pointer"

	usagetime "github.com/upbound/up/internal/usage/time"
)

func TestListBlobsOptionsIterator(t *testing.T) {
	type args struct {
		account string
		tr      usagetime.Range
		window  time.Duration
	}
	type iteration struct {
		ListBlobsOptions []container.ListBlobsFlatOptions
		Window           usagetime.Range
		Err              error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   []iteration
	}{
		"3HourRange1HourWindow": {
			reason: "3h range divided into 1h windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=03/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=04/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=05/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3HourRange2HourWindow": {
			reason: "3h range divided into 2h windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: 2 * time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=03/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=04/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=05/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3HourRange4HourWindow": {
			reason: "3h range divided into 4h windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: 4 * time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=03/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=04/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=05/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3DayRange1DayWindow": {
			reason: "3-day range divided into 1-day windows.",
			args: args{
				account: "test-account",
				tr: usagetime.Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 7, 3, 0, 0, 0, time.UTC),
				},
				window: 24 * time.Hour,
			},
			want: []iteration{
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=03/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=04/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=05/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=06/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=07/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=08/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=09/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=10/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=11/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=12/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=13/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=14/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=15/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=16/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=17/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=18/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=19/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=20/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=21/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=22/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-04/hour=23/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=00/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=01/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=02/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 5, 3, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=03/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=04/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=05/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=06/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=07/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=08/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=09/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=10/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=11/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=12/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=13/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=14/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=15/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=16/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=17/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=18/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=19/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=20/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=21/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=22/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-05/hour=23/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=00/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=01/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=02/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 5, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 6, 3, 0, 0, 0, time.UTC),
					},
				},
				{
					ListBlobsOptions: []container.ListBlobsFlatOptions{
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=03/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=04/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=05/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=06/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=07/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=08/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=09/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=10/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=11/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=12/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=13/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=14/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=15/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=16/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=17/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=18/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=19/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=20/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=21/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=22/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-06/hour=23/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-07/hour=00/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-07/hour=01/")},
						{Prefix: pointer.String("account=test-account/date=2006-05-07/hour=02/")},
					},
					Window: usagetime.Range{
						Start: time.Date(2006, 5, 6, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 7, 3, 0, 0, 0, time.UTC),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			iter, err := NewListBlobsOptionsIterator(tc.args.account, tc.args.tr, tc.args.window)
			if err != nil {
				t.Fatalf("NewListBlobsOptionsIterator() error: %s", err)
			}

			got := []iteration{}
			for iter.More() {
				lbo, window, err := iter.Next()
				got = append(got, iteration{ListBlobsOptions: lbo, Window: window, Err: err})
			}

			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nListBlobsOptionsIterator output: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
