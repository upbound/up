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

package time

import (
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	clock "k8s.io/utils/clock/testing"
)

func TestNewWindowIterator(t *testing.T) {
	type args struct {
		tr     Range
		window time.Duration
	}
	type want struct {
		iter *WindowIterator
		err  error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"59MinuteWindow": {
			reason: "A 59m window should return an error.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
				window: 59 * time.Minute,
			},
			want: want{
				err: errors.New("window must be 1h or greater"),
			},
		},
		"1HourWindow": {
			reason: "A 1h window should be accepted.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
				window: time.Hour,
			},
			want: want{
				iter: &WindowIterator{
					Cursor: clock.SimpleIntervalClock{
						Time:     time.Date(2006, 5, 4, 2, 0, 0, 0, time.UTC),
						Duration: time.Hour,
					},
					End: time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
			},
		},
		"24HourWindow": {
			reason: "A 24h window should be accepted.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
				window: 24 * time.Hour,
			},
			want: want{
				iter: &WindowIterator{
					Cursor: clock.SimpleIntervalClock{
						Time:     time.Date(2006, 5, 3, 3, 0, 0, 0, time.UTC),
						Duration: 24 * time.Hour,
					},
					End: time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
			},
		},
		"30DayWindow": {
			reason: "A 30 day window should be accepted.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
				window: 30 * 24 * time.Hour,
			},
			want: want{
				iter: &WindowIterator{
					Cursor: clock.SimpleIntervalClock{
						Time:     time.Date(2006, 4, 4, 3, 0, 0, 0, time.UTC),
						Duration: 30 * 24 * time.Hour,
					},
					End: time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
			},
		},
		"1HourPrecision": {
			reason: "Times and window should be truncated to the hour.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 4, 2, 1, 0, time.UTC),
				},
				window: time.Hour + time.Minute,
			},
			want: want{
				iter: &WindowIterator{
					Cursor: clock.SimpleIntervalClock{
						Time:     time.Date(2006, 5, 4, 2, 0, 0, 0, time.UTC),
						Duration: time.Hour,
					},
					End: time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			iter, err := NewWindowIterator(tc.args.tr, tc.args.window)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewWindowIterator(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.iter, iter); diff != "" {
				t.Errorf("\n%s\nNewWindowIterator(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWindowIterator(t *testing.T) {
	type args struct {
		tr     Range
		window time.Duration
	}
	type iteration struct {
		Window Range
		Err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   []iteration
	}{
		"3HourRange1HourWindow": {
			reason: "3h range divided into 1h windows.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: time.Hour,
			},
			want: []iteration{
				{
					Window: Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
					},
				},
				{
					Window: Range{
						Start: time.Date(2006, 5, 4, 4, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
					},
				},
				{
					Window: Range{
						Start: time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3HourRange2HourWindow": {
			reason: "3h range divided into 2h windows.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: 2 * time.Hour,
			},
			want: []iteration{
				{
					Window: Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
					},
				},
				{
					Window: Range{
						Start: time.Date(2006, 5, 4, 5, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3HourRange4HourWindow": {
			reason: "3h range divided into 4h windows.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
				},
				window: 4 * time.Hour,
			},
			want: []iteration{
				{
					Window: Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 6, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		"3DayRange1DayWindow": {
			reason: "3-day range divided into 1-day windows.",
			args: args{
				tr: Range{
					Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
					End:   time.Date(2006, 5, 7, 3, 0, 0, 0, time.UTC),
				},
				window: 24 * time.Hour,
			},
			want: []iteration{
				{
					Window: Range{
						Start: time.Date(2006, 5, 4, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 5, 3, 0, 0, 0, time.UTC),
					},
				},
				{
					Window: Range{
						Start: time.Date(2006, 5, 5, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 6, 3, 0, 0, 0, time.UTC),
					},
				},
				{
					Window: Range{
						Start: time.Date(2006, 5, 6, 3, 0, 0, 0, time.UTC),
						End:   time.Date(2006, 5, 7, 3, 0, 0, 0, time.UTC),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			iter, err := NewWindowIterator(tc.args.tr, tc.args.window)
			if err != nil {
				t.Fatalf("NewWindowIterator() error: %s", err)
			}

			got := []iteration{}
			for iter.More() {
				window, err := iter.Next()
				got = append(got, iteration{Window: window, Err: err})
			}

			if diff := cmp.Diff(tc.want, got, test.EquateErrors(), cmpopts.IgnoreUnexported(storage.Query{})); diff != "" {
				t.Errorf("\n%s\nWindowIterator output: -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
