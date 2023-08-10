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

package testing

import (
	"context"
	"fmt"
	"sort"

	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
	"github.com/upbound/up/internal/usage/time"
)

var EOF = event.EOF

// ReadResult is a return value of event.Reader.Read().
type ReadResult struct {
	Event model.MCPGVKEvent
	Err   error
}

var _ event.Reader = &MockReader{}

type MockReader struct {
	Reads []ReadResult
}

func (r *MockReader) Read(context.Context) (model.MCPGVKEvent, error) {
	if len(r.Reads) < 1 {
		return model.MCPGVKEvent{}, EOF
	}
	read := r.Reads[0]
	r.Reads = r.Reads[1:]
	return read.Event, read.Err
}

func (r *MockReader) Close() error {
	return nil
}

// Window is a return value of event.WindowIterator.Next().
type Window struct {
	Reader event.Reader
	Window time.Range
	Err    error
}

var _ event.WindowIterator = &MockWindowIterator{}

type MockWindowIterator struct {
	Windows []Window
}

func (i *MockWindowIterator) More() bool {
	return len(i.Windows) > 0
}

func (i *MockWindowIterator) Next() (event.Reader, time.Range, error) {
	if !i.More() {
		return nil, time.Range{}, fmt.Errorf("iterator is done")
	}
	w := i.Windows[0]
	i.Windows = i.Windows[1:]
	return w.Reader, w.Window, w.Err
}

var _ event.Writer = &MockWriter{}

type MockWriter struct {
	Events []model.MCPGVKEvent
}

func (w *MockWriter) Write(e model.MCPGVKEvent) error {
	if w.Events == nil {
		w.Events = []model.MCPGVKEvent{}
	}
	w.Events = append(w.Events, e)
	return nil
}

// SortEvents sorts events by their fields.
func SortEvents(events []model.MCPGVKEvent) {
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
