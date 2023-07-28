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

package json

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/upbound/up/internal/usage/model"
)

func TestNewMCPGVKEventDecoder(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotJSON": {
			reason: "Creating a decoder from a reader that does not contain JSON should return an error.",
			args: args{
				reader: strings.NewReader("foo"),
			},
			want: want{
				err: errors.New("reader does not contain valid JSON: invalid character 'o' in literal false (expecting 'a')"),
			},
		},
		"NotJSONArray": {
			reason: "Creating a decoder from a reader that does not contain a JSON array should return an error.",
			args: args{
				reader: strings.NewReader("{}"),
			},
			want: want{
				err: errors.New("reader does not contain JSON array. expected [, got {"),
			},
		},
		"EmptyJSONArray": {
			reason: "Creating a decoder from a reader that contains an empty JSON array should not return an error.",
			args: args{
				reader: strings.NewReader("[]"),
			},
			want: want{
				err: nil,
			},
		},
		"PopulatedJSONArray": {
			reason: "Creating a decoder from a reader that contains an empty JSON array should not return an error.",
			args: args{
				reader: strings.NewReader("[{}]"),
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewMCPGVKEventDecoder(tc.args.reader)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewMCPGVKEventDecoder(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMCPGVKEventDecoderMore(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	type want struct {
		more bool
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyJSONArray": {
			reason: "There is nothing more to consume from an empty JSON array.",
			args: args{
				reader: strings.NewReader("[]"),
			},
			want: want{
				more: false,
			},
		},
		"PopulatedJSONArray": {
			reason: "There is more to consume from a populated JSON array.",
			args: args{
				reader: strings.NewReader("[{}]"),
			},
			want: want{
				more: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := NewMCPGVKEventDecoder(tc.args.reader)
			if err != nil {
				diff := cmp.Diff(nil, err, test.EquateErrors())
				t.Errorf("\n%s\nNewMCPGVKEventDecoder(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			more := d.More()
			if diff := cmp.Diff(tc.want.more, more); diff != "" {
				t.Errorf("\n%s\nMCPGVKEventDecoder.More(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMCPGVKEventDecoderDecode(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	type want struct {
		event model.MCPGVKEvent
		err   error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyJSONArray": {
			reason: "Decoding from an empty JSON array should return an error.",
			args: args{
				reader: strings.NewReader("[]"),
			},
			want: want{
				event: model.MCPGVKEvent{},
				err:   errors.New("error decoding next event: invalid character ']' looking for beginning of value"),
			},
		},
		"InvalidUsageObject": {
			reason: "Decoding from a JSON array containing an invalid usage object should return an error.",
			args: args{
				reader: strings.NewReader("[{\"value\": \"a string\"}]"),
			},
			want: want{
				event: model.MCPGVKEvent{},
				err:   errors.New("error decoding next event: json: cannot unmarshal string into Go struct field MCPGVKEvent.value of type float64"),
			},
		},
		"EmptyUsageObject": {
			reason: "Decoding from a JSON array containing an empty usage object should return an uninitialized MCPGVKEvent.",
			args: args{
				reader: strings.NewReader("[{}]"),
			},
			want: want{
				event: model.MCPGVKEvent{},
				err:   nil,
			},
		},
		"UsageObject": {
			reason: "Decoding from a JSON array containing a usage object should return aMCPGVKEvent with its values.",
			args: args{
				reader: strings.NewReader(`
[{
  "kind": "absolute",
  "name": "event_name",
  "tags": {
    "customresource_group": "example.com",
    "customresource_version": "v1",
    "customresource_kind": "Thing",
    "mcp_id": "test-mcp-id",
    "upbound_account": "test-account"
  },
  "timestamp": "2023-03-16T00:00:00.0Z",
  "timestamp_end": "2023-03-16T00:00:00.0Z",
  "value": 1.0
}]`),
			},
			want: want{
				event: model.MCPGVKEvent{
					Name:  "event_name",
					Value: 1.0,
					Tags: model.MCPGVKEventTags{
						MCPID:          "test-mcp-id",
						UpboundAccount: "test-account",
						Group:          "example.com",
						Version:        "v1",
						Kind:           "Thing",
					},
					Timestamp:    time.Date(2023, time.March, 16, 0, 0, 0, 0, time.UTC),
					TimestampEnd: time.Date(2023, time.March, 16, 0, 0, 0, 0, time.UTC),
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := NewMCPGVKEventDecoder(tc.args.reader)
			if err != nil {
				diff := cmp.Diff(nil, err, test.EquateErrors())
				t.Errorf("\n%s\nNewMCPGVKEventDecoder(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			e, err := d.Decode()
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMCPGVKEventDecoder.Decode(): -want err, +got err:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.event, e); diff != "" {
				t.Errorf("\n%s\nMCPGVKEventDecoder.Decode(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
