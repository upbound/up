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

package report

import (
	"context"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/aggregate"
	"github.com/upbound/up/internal/usage/event"
	usagetime "github.com/upbound/up/internal/usage/time"
)

const (
	errReadEvents  = "error reading events"
	errWriteEvents = "error writing events"
)

// Meta contains metadata for a usage report.
type Meta struct {
	UpboundAccount string          `json:"account"`
	TimeRange      usagetime.Range `json:"time_range"`
	CollectedAt    time.Time       `json:"collected_at"`
}

// MaxResourceCountPerGVKPerMCP reads events from i and writes aggregated events
// to w. Events are aggregated across each window of time returned by i. An
// aggregated event records the largest observed count of instances of a GVK on
// an MCP during a window. The order of written events is not stable.
func MaxResourceCountPerGVKPerMCP(ctx context.Context, i event.WindowIterator, w event.Writer) error {
	for i.More() {
		r, window, err := i.Next()
		if err != nil {
			return errors.Wrap(err, errReadEvents)
		}

		ag := &aggregate.MaxResourceCountPerGVKPerMCP{}
		for {
			e, err := r.Read(ctx)
			if errors.Is(err, event.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if err := ag.Add(e); err != nil {
				return err
			}
		}
		if err := r.Close(); err != nil {
			return errors.Wrap(err, errReadEvents)
		}

		for _, e := range ag.UpboundEvents() {
			e.Timestamp = window.Start
			e.TimestampEnd = window.End
			if err := w.Write(e); err != nil {
				return errors.Wrap(err, errWriteEvents)
			}
		}
	}
	return nil
}
