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
	"fmt"
	"time"

	clock "k8s.io/utils/clock/testing"
)

type Range struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FormatDateUTC returns t in UTC as a string with the format YYYY-MM-DD.
func FormatDateUTC(t time.Time) string {
	return t.UTC().Format(time.DateOnly)
}

// WindowIterator iterates through windows of a range of time. Must be
// initialized with NewWindowIterator().
type WindowIterator struct {
	Cursor clock.SimpleIntervalClock
	End    time.Time
}

// NewWindowIterator returns an initialized *WindowIterator.
func NewWindowIterator(tr Range, window time.Duration) (*WindowIterator, error) {
	if window < time.Hour {
		return nil, fmt.Errorf("window must be 1h or greater")
	}
	if tr.End.Before(tr.Start.Add(time.Hour)) {
		return nil, fmt.Errorf("time range must be at least 1h")
	}
	tr.Start = tr.Start.Truncate(time.Hour)
	tr.End = tr.End.Truncate(time.Hour)
	window = window.Truncate(time.Hour)
	return &WindowIterator{
		Cursor: clock.SimpleIntervalClock{
			// Initialize the clock early by one window to account for the first
			// Now() call advancing it by one window.
			Time:     tr.Start.Add(-1 * window),
			Duration: window,
		},
		End: tr.End,
	}, nil
}

// More() returns true if Next() has more to return.
func (i *WindowIterator) More() bool {
	// If the cursor is before the end time by at least one window, then there's
	// at least one more window to return from Next().
	return i.Cursor.Since(i.End) < (-1 * i.Cursor.Duration)
}

// Next() returns a time range covering the next window of time. The start
// time is inclusive and the end time is exclusive. Returns an error if More()
// returns false.
func (i *WindowIterator) Next() (Range, error) {
	if !i.More() {
		return Range{}, fmt.Errorf("iterator is done")
	}
	start := i.Cursor.Now()
	window := Range{Start: start, End: start.Add(i.Cursor.Duration)}
	if window.End.After(i.End) {
		window.End = i.End
	}
	return window, nil
}
