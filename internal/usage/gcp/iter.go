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

package gcp

import (
	"fmt"
	"time"

	"cloud.google.com/go/storage"

	"github.com/upbound/up/internal/usage/event"
	usagetime "github.com/upbound/up/internal/usage/time"
)

var _ event.WindowIterator = &WindowIterator{}

// WindowIterator iterates through readers for windows of usage events from an
// Azure blob storage container. Must be initialized with NewWindowIterator().
type WindowIterator struct {
	Iter   *QueryIterator
	Bucket *storage.BucketHandle
}

// NewWindowIterator returns an initialized *WindowIterator.
func NewWindowIterator(bkt *storage.BucketHandle, account string, tr usagetime.Range, window time.Duration) (*WindowIterator, error) {
	iter, err := NewQueryIterator(account, tr, window)
	if err != nil {
		return nil, err
	}
	return &WindowIterator{
		Iter:   iter,
		Bucket: bkt,
	}, nil
}

func (i *WindowIterator) More() bool {
	return i.Iter.More()
}

func (i *WindowIterator) Next() (event.Reader, usagetime.Range, error) {
	query, window, err := i.Iter.Next()
	if err != nil {
		return nil, usagetime.Range{}, err
	}
	return &QueryEventReader{Bucket: i.Bucket, Query: query}, window, nil
}

// QueryIterator iterates through queries for usage data for an Upbound
// account across a range of time. Each query covers a window of time within the
// time range. Must be initialized with NewUsageQueryIterator().
type QueryIterator struct {
	Account string
	Iter    *usagetime.WindowIterator
}

// NewQueryIterator() returns an initialized *UsageQueryIterator. The
// start of the time range is inclusive and the end is exclusive to the hour.
// The time range and window are truncated to the hour.
func NewQueryIterator(account string, tr usagetime.Range, window time.Duration) (*QueryIterator, error) {
	iter, err := usagetime.NewWindowIterator(tr, window)
	if err != nil {
		return nil, err
	}
	return &QueryIterator{
		Account: account,
		Iter:    iter,
	}, nil
}

// More() returns true if Next() has more queries to return.
func (i *QueryIterator) More() bool {
	return i.Iter.More()
}

// Next() returns a query covering the next window of time, as well as a pair
// of times marking the start and end of the window.
func (i *QueryIterator) Next() (*storage.Query, usagetime.Range, error) {
	window, err := i.Iter.Next()
	if err != nil {
		return nil, usagetime.Range{}, err
	}
	return query(i.Account, window), window, nil
}

// Query() returns a query for usage data for an Upbound account across a
// range of time. The start of the range is inclusive and the end is exclusive
// to the hour.
func Query(account string, tr usagetime.Range) (*storage.Query, error) {
	if tr.End.Before(tr.Start) {
		return nil, fmt.Errorf("time range must start before it ends")
	}
	return query(account, tr), nil
}

func query(account string, tr usagetime.Range) *storage.Query {
	return &storage.Query{
		StartOffset: fmt.Sprintf(
			"account=%s/date=%s/hour=%02d/",
			account,
			usagetime.FormatDateUTC(tr.Start),
			tr.Start.Hour(),
		),
		EndOffset: fmt.Sprintf(
			"account=%s/date=%s/hour=%02d/",
			account,
			usagetime.FormatDateUTC(tr.End),
			tr.End.Hour(),
		),
	}
}
