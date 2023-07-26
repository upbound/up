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
	"sort"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/upbound/up/internal/usage/model"
)

func TestMaxResourceCountPerGVKPerMCPAdd(t *testing.T) {
	type args struct {
		event model.MCPGVKEvent
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
				event: model.MCPGVKEvent{
					Name: "unexpected_name",
					Tags: model.MCPGVKEventTags{
						MCPID:   "test-mcp-id",
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
		"EmptyMCPID": {
			reason: "Adding an event with an empty MCPID should return an error.",
			args: args{
				event: model.MCPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MCPGVKEventTags{
						MCPID:   "",
						Group:   "example.com",
						Version: "v1",
						Kind:    "Thing",
					},
				},
			},
			want: want{
				err: errors.New("MCPID tag is empty"),
			},
		},
		"EmptyGroup": {
			reason: "Adding an event with an empty Group should return an error.",
			args: args{
				event: model.MCPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MCPGVKEventTags{
						MCPID:   "test-mcp-id",
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
				event: model.MCPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MCPGVKEventTags{
						MCPID:   "test-mcp-id",
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
				event: model.MCPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MCPGVKEventTags{
						MCPID:   "test-mcp-id",
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
				event: model.MCPGVKEvent{
					Name: "kube_managedresource_uid",
					Tags: model.MCPGVKEventTags{
						MCPID:   "test-mcp-id",
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
			ag := MaxResourceCountPerGVKPerMCP{}
			err := ag.Add(tc.args.event)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMaxResourceCountPerGVKPerMCP.Add(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMaxResouceCountPerGVKPerMCPUpboundEvents(t *testing.T) {
	type args struct {
		events []model.MCPGVKEvent
	}
	type want struct {
		events []model.MCPGVKEvent
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoEvents": {
			reason: "There should be no events emitted if none were added.",
			args: args{
				events: []model.MCPGVKEvent{},
			},
			want: want{
				events: []model.MCPGVKEvent{},
			},
		},
		"UseLargestValue": {
			reason: "Upbound events should use the largest added value for a resource count on an MCP.",
			args: args{
				events: []model.MCPGVKEvent{
					{
						Name:  "kube_managedresource_uid",
						Value: 8.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 10.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 2.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
				},
			},
			want: want{
				events: []model.MCPGVKEvent{
					{
						Name:  "max_resource_count_per_gvk_per_mcp",
						Value: 10.0, // largest added value
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
				},
			},
		},
		"EventPerMCPGVK": {
			reason: "Different events should be emitted for different combinations of MCP and GVK.",
			args: args{
				events: []model.MCPGVKEvent{
					{
						Name:  "kube_managedresource_uid",
						Value: 4.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id-1",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 5.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id-2",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "kube_managedresource_uid",
						Value: 6.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id-1",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Item",
						},
					},
				},
			},
			want: want{
				events: []model.MCPGVKEvent{
					{
						Name:  "max_resource_count_per_gvk_per_mcp",
						Value: 4.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id-1",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "max_resource_count_per_gvk_per_mcp",
						Value: 5.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id-2",
							Group:   "example.com",
							Version: "v1",
							Kind:    "Thing",
						},
					},
					{
						Name:  "max_resource_count_per_gvk_per_mcp",
						Value: 6.0,
						Tags: model.MCPGVKEventTags{
							MCPID:   "test-mcp-id-1",
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
			ag := MaxResourceCountPerGVKPerMCP{}
			for i, e := range tc.args.events {
				if err := ag.Add(e); err != nil {
					diff := cmp.Diff(nil, err, test.EquateErrors())
					t.Errorf("\n%s\nMaxResourceCountPerGVKPerMCP.Add(...): error adding event %d: -want err, +got err:\n%s", tc.reason, i, diff)
				}
			}

			got := ag.UpboundEvents()

			// Sort for stability.
			sortUpboundEvents(got)
			sortUpboundEvents(tc.want.events)

			if diff := cmp.Diff(tc.want.events, got); diff != "" {
				t.Errorf("\n%s\nMaxResourceCountPerGVKPerMCP.UpboundEvents(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// sortUpboundEvents sorts Upbound events by their fields.
func sortUpboundEvents(events []model.MCPGVKEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Name != events[j].Name {
			return events[i].Name < events[j].Name
		}
		if events[i].Tags.UpboundAccount != events[j].Tags.UpboundAccount {
			return events[i].Tags.UpboundAccount < events[j].Tags.UpboundAccount
		}
		if events[i].Tags.MCPID != events[j].Tags.MCPID {
			return events[i].Tags.MCPID < events[j].Tags.MCPID
		}
		if events[i].Tags.Group != events[j].Tags.Group {
			return events[i].Tags.Group < events[j].Tags.Group
		}
		if events[i].Tags.Version != events[j].Tags.Version {
			return events[i].Tags.Version < events[j].Tags.Version
		}
		if events[i].Tags.Kind != events[j].Tags.Kind {
			return events[i].Tags.Kind < events[j].Tags.Kind
		}
		return events[i].Value < events[j].Value
	})
}
