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

package event

import (
	"context"
	"errors"

	"github.com/upbound/up/internal/usage/model"
	"github.com/upbound/up/internal/usage/time"
)

var ErrEOF = errors.New("EOF")

// Reader is the interface for reading usage events. Read() must return EOF when
// there is nothing more to read. Callers must call Close() when finished
// reading.
type Reader interface {
	// Read returns the next event. Returns EOF when finished.
	Read(context.Context) (model.MCPGVKEvent, error)
	// Close closes the reader.
	Close() error
}

// WindowIterator is the interface for iterating through usage event readers for
// windows of time within a time range.
type WindowIterator interface {
	// More returns true if there are more windows.
	More() bool
	// Next returns a reader and time range for the next window.
	Next() (Reader, time.Range, error)
}

// Writer is the interface for reading usage events.
type Writer interface {
	// Write writes an event.
	Write(model.MCPGVKEvent) error
}
