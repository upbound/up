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

package reader

import (
	"context"
	"fmt"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
	usagetesting "github.com/upbound/up/internal/usage/testing"
)

func TestMultiReader(t *testing.T) {
	cases := map[string]struct {
		reason string
		reader MultiReader
		want   []usagetesting.ReadResult
	}{
		"Uninitialized": {
			reason: "An unitialized MultiReader returns no events.",
			reader: MultiReader{},
			want:   []usagetesting.ReadResult{},
		},
		"SingleReader": {
			reason: "Events from a single reader are returned in order.",
			reader: MultiReader{Readers: []event.Reader{
				&usagetesting.MockReader{Reads: []usagetesting.ReadResult{
					{Event: model.MCPGVKEvent{Name: "event-1"}},
					{Event: model.MCPGVKEvent{Name: "event-2"}},
					{Event: model.MCPGVKEvent{Name: "event-3"}},
				}},
			}},
			want: []usagetesting.ReadResult{
				{Event: model.MCPGVKEvent{Name: "event-1"}},
				{Event: model.MCPGVKEvent{Name: "event-2"}},
				{Event: model.MCPGVKEvent{Name: "event-3"}},
			},
		},
		"SingleEmptyReader": {
			reason: "No events are returned from a single empty reader.",
			reader: MultiReader{Readers: []event.Reader{
				&usagetesting.MockReader{},
			}},
			want: []usagetesting.ReadResult{},
		},
		"MultipleReaders": {
			reason: "Events from multiple readers are returned in order.",
			reader: MultiReader{Readers: []event.Reader{
				&usagetesting.MockReader{Reads: []usagetesting.ReadResult{
					{Event: model.MCPGVKEvent{Name: "event-1"}},
				}},
				&usagetesting.MockReader{Reads: []usagetesting.ReadResult{
					{Event: model.MCPGVKEvent{Name: "event-2"}},
					{Event: model.MCPGVKEvent{Name: "event-3"}},
				}},
			}},
			want: []usagetesting.ReadResult{
				{Event: model.MCPGVKEvent{Name: "event-1"}},
				{Event: model.MCPGVKEvent{Name: "event-2"}},
				{Event: model.MCPGVKEvent{Name: "event-3"}},
			},
		},
		"MultipleEmptyReaders": {
			reason: "No events are returned from multiple empty readers.",
			reader: MultiReader{Readers: []event.Reader{
				&usagetesting.MockReader{},
				&usagetesting.MockReader{},
			}},
			want: []usagetesting.ReadResult{},
		},
		"Error": {
			reason: "An error from a reader is returned.",
			reader: MultiReader{Readers: []event.Reader{
				&usagetesting.MockReader{Reads: []usagetesting.ReadResult{
					{Event: model.MCPGVKEvent{Name: "event-1"}},
					{Err: fmt.Errorf("boom")},
				}},
			}},
			want: []usagetesting.ReadResult{
				{Event: model.MCPGVKEvent{Name: "event-1"}},
				{Err: fmt.Errorf("boom")},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			got := []usagetesting.ReadResult{}
			for {
				e, err := tc.reader.Read(ctx)
				// Stop reading if error is EOF. If error is otherwise non-nil,
				// add result to got and then stop reading. If error is nil, add
				// result to got and continue reading.
				if errors.Is(err, EOF) {
					break
				}
				got = append(got, usagetesting.ReadResult{Event: e, Err: err})
				if err != nil {
					break
				}
			}

			err := tc.reader.Close()
			if diff := cmp.Diff(nil, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMultiReader.Close(): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMultiReader: -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
