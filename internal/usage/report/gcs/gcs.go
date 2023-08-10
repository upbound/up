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

package gcs

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	gcpopt "google.golang.org/api/option"

	"github.com/upbound/up/internal/usage/aggregate"
	"github.com/upbound/up/internal/usage/clientutil/gcs"
	"github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/event"
	usagetime "github.com/upbound/up/internal/usage/time"
)

const (
	// Number of objects to read concurrently.
	concurrency = 10

	errReadEvents  = "error reading events"
	errWriteEvents = "error writing events"
)

// GenerateReport initializes the client code and generates a usage report based on given inputs
func GenerateReport(ctx context.Context, account, endpoint, bucket string, billingPeriod usagetime.Range, window time.Duration, w event.Writer) error {
	opts := []gcpopt.ClientOption{}
	if endpoint != "" {
		opts = append(opts, gcpopt.WithEndpoint(endpoint))
	}
	gcsCli, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "error creating storage client")
	}
	bkt := gcsCli.Bucket(bucket)
	if err := maxResourceCountPerGVKPerMCP(ctx, account, bkt, billingPeriod, time.Hour, w); err != nil {
		return err
	}
	return nil
}

// maxResourceCountPerGVKPerMCP reads usage data for an account and time range
// from bkt and writes aggregated usage events to w. Events are aggregated
// across each window of the time range.
func maxResourceCountPerGVKPerMCP(ctx context.Context, account string, bkt *storage.BucketHandle, tr usagetime.Range, window time.Duration, w event.Writer) error {
	// TODO(branden): Extract provider-generic upbound event reader interface so
	// that this function can be reused across providers.
	iter, err := gcs.NewUsageQueryIterator(account, tr.Start, tr.End, window)
	if err != nil {
		return errors.Wrap(err, errReadEvents)
	}

	for iter.More() {
		query, start, end, err := iter.Next()
		if err != nil {
			return errors.Wrap(err, errReadEvents)
		}
		objects := bkt.Objects(ctx, query)

		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		ag := &aggregate.MaxResourceCountPerGVKPerMCP{}
		agMu := &sync.Mutex{}

		for {
			attrs, err := objects.Next()
			if errors.Is(err, iterator.Done) {
				break
			}

			obj := bkt.Object(attrs.Name)
			g.Go(func() error {
				return readObject(ctx, ag, agMu, obj)
			})
		}
		if err := g.Wait(); err != nil {
			return errors.Wrap(err, errReadEvents)
		}

		for _, e := range ag.UpboundEvents() {
			e.Timestamp = start
			e.TimestampEnd = end
			if err := w.Write(e); err != nil {
				return errors.Wrap(err, errWriteEvents)
			}
		}
	}
	return nil
}

// readObject() decodes MCP GVK events from an object and adds them to an aggregate.
func readObject(ctx context.Context, ag *aggregate.MaxResourceCountPerGVKPerMCP, agMu sync.Locker, obj *storage.ObjectHandle) error {
	r, err := obj.NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close() // nolint:errcheck

	d, err := json.NewMCPGVKEventDecoder(r)
	if err != nil {
		return err
	}

	for d.More() {
		e, err := d.Decode()
		if err != nil {
			return err
		}

		agMu.Lock()
		err = ag.Add(e)
		agMu.Unlock()

		if err != nil {
			return err
		}
	}
	return nil
}
