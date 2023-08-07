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

package clientutil

import (
	"fmt"
	"time"
)

func usageQueryValues(account string, startTime, endTime time.Time) (startPrefix, endPrefix string) {
	return fmt.Sprintf(
			"account=%s/date=%s/hour=%02d/",
			account,
			formatDateUTC(startTime),
			startTime.Hour(),
		),
		fmt.Sprintf(
			"account=%s/date=%s/hour=%02d/",
			account,
			formatDateUTC(endTime),
			endTime.Hour(),
		)
}

// UsageQueryIterator iterates through queries for usage data for an Upbound
// account across a range of time. Each query covers a window of time within the
// time range. Must be initialized with NewUsageQueryIterator().
type UsageQueryIterator struct {
	Account string
	Cursor  time.Time
	EndTime time.Time
	Window  time.Duration
}

// NewUsageQueryIterator() returns an initialized *UsageQueryIterator.
// startTime is inclusive and endTime is exclusive to the hour. startTime,
// endTime, and window are truncated to the hour.
func NewUsageQueryIterator(account string, startTime, endTime time.Time, window time.Duration) (*UsageQueryIterator, error) {
	if window < time.Hour {
		return nil, fmt.Errorf("window must be 1h or greater")
	}
	if endTime.Before(startTime.Add(time.Hour)) {
		return nil, fmt.Errorf("endTime must occur at least 1h after startTime")
	}
	startTime = startTime.Truncate(time.Hour)
	endTime = endTime.Truncate(time.Hour)
	window = window.Truncate(time.Hour)
	return &UsageQueryIterator{
		Account: account,
		Cursor:  startTime,
		EndTime: endTime,
		Window:  window,
	}, nil
}

// More() returns true if Next() has more queries to return.
func (i *UsageQueryIterator) More() bool {
	return i.Cursor.Before(i.EndTime)
}

// Next() returns a query covering the next window of time, as well as a pair
// of times marking the start and end of the window.
func (i *UsageQueryIterator) Next() (string, string, time.Time, time.Time, error) {
	if !i.More() {
		return "", "", time.Time{}, time.Time{}, fmt.Errorf("iterator is done")
	}
	start := i.Cursor
	i.Cursor = i.Cursor.Add(i.Window)
	if i.Cursor.After(i.EndTime) {
		i.Cursor = i.EndTime
	}
	startPrefix, endPrefix := usageQueryValues(i.Account, start, i.Cursor)
	return startPrefix, endPrefix, start, i.Cursor, nil
}

// formatDateUTC returns t in UTC as a string with the format YYYY-MM-DD.
func formatDateUTC(t time.Time) string {
	return t.UTC().Format(time.DateOnly)
}
