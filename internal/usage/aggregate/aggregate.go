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
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/model"
)

const (
	mrCountUpboundEventName    = "kube_managedresource_uid"
	mrCountMaxUpboundEventName = "max_resource_count_per_gvk_per_mcp"
)

type mcpGVK struct {
	MCPID   string
	Group   string
	Version string
	Kind    string
}

// MaxResourceCountPerGVKPerMCP aggregates the maximum recorded GVK counts per MCP from
// Upbound usage events.
type MaxResourceCountPerGVKPerMCP struct {
	counts map[mcpGVK]int
}

// Add adds a usage event to the aggregate.
func (ag *MaxResourceCountPerGVKPerMCP) Add(e model.MCPGVKEvent) error {
	if err := ag.validateEvent(e); err != nil {
		return err
	}

	value := int(e.Value)
	key := mcpGVK{
		MCPID:   e.Tags.MCPID,
		Group:   e.Tags.Group,
		Version: e.Tags.Version,
		Kind:    e.Tags.Kind,
	}

	if ag.counts == nil {
		ag.counts = make(map[mcpGVK]int)
	}
	if value > ag.counts[key] {
		ag.counts[key] = value
	}

	return nil
}

// UpboundEvents returns an Upbound usage event for each combination of MCP and
// GVK.
func (ag *MaxResourceCountPerGVKPerMCP) UpboundEvents() []model.MCPGVKEvent {
	events := []model.MCPGVKEvent{}
	for key, count := range ag.counts {
		events = append(events, model.MCPGVKEvent{
			Name:  mrCountMaxUpboundEventName,
			Value: float64(count),
			Tags: model.MCPGVKEventTags{
				MCPID:   key.MCPID,
				Group:   key.Group,
				Version: key.Version,
				Kind:    key.Kind,
			},
		})
	}
	return events
}

func (ag *MaxResourceCountPerGVKPerMCP) validateEvent(e model.MCPGVKEvent) error {
	if e.Name != mrCountUpboundEventName {
		return fmt.Errorf("expected event name %s, got %s", mrCountUpboundEventName, e.Name)
	}
	if e.Tags.MCPID == "" {
		return errors.New("MCPID tag is empty")
	}
	if e.Tags.Group == "" {
		return errors.New("Group tag is empty")
	}
	if e.Tags.Version == "" {
		return errors.New("Version tag is empty")
	}
	if e.Tags.Kind == "" {
		return errors.New("Kind tag is empty")
	}
	return nil

}
