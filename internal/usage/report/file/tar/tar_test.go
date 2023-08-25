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

package tar

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/upbound/up/internal/usage/model"
	"github.com/upbound/up/internal/usage/report"
	usagetime "github.com/upbound/up/internal/usage/time"
)

// makeTestData creates the test data used by TestWriter(). This function is
// not called anywhere. It is made available here as documentation of how test
// data was created and for use when updating tests.
func makeTestData() { // nolint:unused
	panicOnErr := func(err error, msg string) {
		if err != nil {
			panic(msg)
		}
	}

	meta := report.Meta{
		UpboundAccount: "test-account",
		TimeRange: usagetime.Range{
			Start: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
			End:   time.Date(2006, 5, 4, 4, 2, 1, 0, time.UTC),
		},
		CollectedAt: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
	}

	func() {
		filename, _ := filepath.Abs("testdata/empty.tar")
		f, err := os.Create(filename)
		panicOnErr(err, "os.Create()")
		defer f.Close()
		tw := tar.NewWriter(f)
		defer tw.Close()
		rw, err := NewWriter(tw, meta)
		panicOnErr(err, "NewWriter()")
		defer rw.Close()
	}()

	func() {
		filename, _ := filepath.Abs("testdata/example.tar")
		f, err := os.Create(filename)
		panicOnErr(err, "os.Create()")
		defer f.Close()
		tw := tar.NewWriter(f)
		defer tw.Close()
		rw, err := NewWriter(tw, meta)
		panicOnErr(err, "NewWriter()")
		defer rw.Close()

		events := []model.MXPGVKEvent{
			{},
			{},
			{},
			{},
			{},
			{},
			{},
		}
		for _, e := range events {
			err := rw.Write(e)
			panicOnErr(err, "Writer.Write()")
		}
	}()
}

func TestWriter(t *testing.T) {
	type args struct {
		meta   report.Meta
		events []model.MXPGVKEvent
	}
	type want struct {
		file string
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoEvents": {
			reason: "Writer is closed without writing any events.",
			args: args{
				meta: report.Meta{
					UpboundAccount: "test-account",
					TimeRange: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 4, 2, 1, 0, time.UTC),
					},
					CollectedAt: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
				},
				events: []model.MXPGVKEvent{},
			},
			want: want{
				file: "testdata/empty.tar",
			},
		},
		"MultipleEvents": {
			reason: "Writer is closed after writing multiple events.",
			args: args{
				meta: report.Meta{
					UpboundAccount: "test-account",
					TimeRange: usagetime.Range{
						Start: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
						End:   time.Date(2006, 5, 4, 4, 2, 1, 0, time.UTC),
					},
					CollectedAt: time.Date(2006, 5, 4, 3, 2, 1, 0, time.UTC),
				},
				events: []model.MXPGVKEvent{
					{},
					{},
					{},
					{},
					{},
					{},
					{},
				},
			},
			want: want{
				file: "testdata/example.tar",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tw := tar.NewWriter(buf)
			rw, err := NewWriter(tw, tc.args.meta)
			if err != nil {
				diff := cmp.Diff(nil, err, test.EquateErrors())
				t.Errorf("\n%s\nNewWriter(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			for _, e := range tc.args.events {
				err := rw.Write(e)
				if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nWriter.Write(...): -want err, +got err:\n%s", tc.reason, diff)
				}
			}

			err = rw.Close()
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nWriter.Close(): -want err, +got err:\n%s", tc.reason, diff)
			}
			err = tw.Close()
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ntar.Writer.Close(): -want err, +got err:\n%s", tc.reason, diff)
			}

			want, err := os.ReadFile(tc.want.file)
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nos.ReadFile(...): -want, +got:\n%s", tc.reason, diff)
			}

			got := buf.Bytes()
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("\n%s\nWriter: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
