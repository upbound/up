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

package aggregate

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/upbound/up/internal/usage/model"
	usagetesting "github.com/upbound/up/internal/usage/testing"
)

func TestMaxResourceCountPerGVKPerMXPAdd(t *testing.T) {
	type args struct {
		event model.MXPGVKEvent
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnexpectedName": {
			reason: "Adding an event with an unexpected name should return an error.",
			args: args{
				event: model.MXPGVKEvent{
					Name: "unexpected_name",
					Tags: model.MXPGVKEventTags{
						MXPID:   "test-mxp-id",
						Group:   "example.com",
						Version: "v1",
						Kind:    "Thing",
					},
				},
			},
			want: want{
				err: errors.New("expected event name kube_managedresource_uid, got unexpected_name"),
			},
		},
		"EmptyMXPID": {
			reason: "Adding an event with an empty MXPID should return an error.",
			args: args{
				event: model.MXPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MXPGVKEventTags{
						MXPID:   "",
						Group:   "example.com",
						Version: "v1",
						Kind:    "Thing",
					},
				},
			},
			want: want{
				err: errors.New("MXPID tag is empty"),
			},
		},
		"EmptyGroup": {
			reason: "Adding an event with an empty Group should return an error.",
			args: args{
				event: model.MXPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MXPGVKEventTags{
						MXPID:   "test-mxp-id",
						Group:   "",
						Version: "v1",
						Kind:    "Thing",
					},
				},
			},
			want: want{
				err: errors.New("Group tag is empty"),
			},
		},
		"EmptyVersion": {
			reason: "Adding an event with an empty Version should return an error.",
			args: args{
				event: model.MXPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MXPGVKEventTags{
						MXPID:   "test-mxp-id",
						Group:   "example.com",
						Version: "",
						Kind:    "Thing",
					},
				},
			},
			want: want{
				err: errors.New("Version tag is empty"),
			},
		},
		"EmptyKind": {
			reason: "Adding an event with an empty Kind should return an error.",
			args: args{
				event: model.MXPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MXPGVKEventTags{
						MXPID:   "test-mxp-id",
						Group:   "example.com",
						Version: "v1",
						Kind:    "",
					},
				},
			},
			want: want{
				err: errors.New("Kind tag is empty"),
			},
		},
		"ValidEvent": {
			reason: "Adding a valid event should return a nil error.",
			args: args{
				event: model.MXPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MXPGVKEventTags{
						MXPID:   "test-mxp-id",
						Group:   "example.com",
						Version: "v1",
						Kind:    "Thing",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ag := MaxResourceCountPerGVKPerMXP{}
			err := ag.Add(tc.args.event)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMaxResourceCountPerGVKPerMXP.Add(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMaxResouceCountPerGVKPerMXPUpboundEvents(t *testing.T) {
	type args struct {
		events []model.MXPGVKEvent
	}
	type want struct {
		events []model.MXPGVKEvent
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoEvents": {
			reason: "There should be no events emitted if none were added.",
			args: args{
				events: []model.MXPGVKEvent{},
			},
			want: want{
				events: []model.MXPGVKEvent{},
			},
		},
		"UseLargestValue": {
			reason: "Upbound events should use the largest added value for a resource count on an MXP.",
			args: args{
				events: []model.MXPGVKEvent{
					{
						Name:  "kube_managedresource_uid",
						Value: 8.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 10.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 2.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
				},
			},
			want: want{
				events: []model.MXPGVKEvent{
					{
						Name:  "max_resource_count_per_gvk_per_mxp",
						Value: 10.0, // largest added value
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
				},
			},
		},
		"EventPerMXPGVK": {
			reason: "Different events should be emitted for different combinations of MXP and GVK.",
			args: args{
				events: []model.MXPGVKEvent{
					{
						Name:  "kube_managedresource_uid",
						Value: 4.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id-1",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 5.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id-2",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 6.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id-1",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Item",
						},
					},
				},
			},
			want: want{
				events: []model.MXPGVKEvent{
					{
						Name:  "max_resource_count_per_gvk_per_mxp",
						Value: 4.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id-1",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "max_resource_count_per_gvk_per_mxp",
						Value: 5.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id-2",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "max_resource_count_per_gvk_per_mxp",
						Value: 6.0,
						Tags: model.MXPGVKEventTags{
							MXPID:   "test-mxp-id-1",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Item",
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ag := MaxResourceCountPerGVKPerMXP{}
			for i, e := range tc.args.events {
				if err := ag.Add(e); err != nil {
					diff := cmp.Diff(nil, err, test.EquateErrors())
					t.Errorf("\n%s\nMaxResourceCountPerGVKPerMXP.Add(...): error adding event %d: -want err, +got err:\n%s", tc.reason, i, diff)
				}
			}

			got := ag.UpboundEvents()

			// Sort for stability.
			usagetesting.SortEvents(got)
			usagetesting.SortEvents(tc.want.events)

			if diff := cmp.Diff(tc.want.events, got); diff != "" {
				t.Errorf("\n%s\nMaxResourceCountPerGVKPerMXP.UpboundEvents(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
