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

	usagetime "github.com/upbound/up/internal/usage/time"
)

func usageQueryValues(account string, tr usagetime.Range) (startPrefix, endPrefix string) {
	return fmt.Sprintf(
			"account=%s/date=%s/hour=%02d/",
			account,
			usagetime.FormatDateUTC(tr.Start),
			tr.Start.Hour(),
		),
		fmt.Sprintf(
			"account=%s/date=%s/hour=%02d/",
			account,
			usagetime.FormatDateUTC(tr.End),
			tr.End.Hour(),
		)
}

// UsageQueryIterator iterates through queries for usage data for an Upbound
// account across a range of time. Each query covers a window of time within the
// time range. Must be initialized with NewUsageQueryIterator().
type UsageQueryIterator struct {
	Account string
	Iter    *usagetime.WindowIterator
}

// NewUsageQueryIterator() returns an initialized *UsageQueryIterator.
func NewUsageQueryIterator(account string, tr usagetime.Range, window time.Duration) (*UsageQueryIterator, error) {
	iter, err := usagetime.NewWindowIterator(tr, window)
	if err != nil {
		return nil, err
	}
	return &UsageQueryIterator{
		Account: account,
		Iter:    iter,
	}, nil
}

// More() returns true if Next() has more queries to return.
func (i *UsageQueryIterator) More() bool {
	return i.Iter.More()
}

// Next() returns a query covering the next window of time, as well as a time
// range marking the window.
func (i *UsageQueryIterator) Next() (string, string, usagetime.Range, error) {
	window, err := i.Iter.Next()
	if err != nil {
		return "", "", usagetime.Range{}, err
	}

	startPrefix, endPrefix := usageQueryValues(i.Account, window)
	return startPrefix, endPrefix, window, nil
}
