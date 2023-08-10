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

package aws

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/upbound/up/internal/usage/aggregate"
	"github.com/upbound/up/internal/usage/clientutil"
	"github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/event"
	usagetime "github.com/upbound/up/internal/usage/time"
)

const (
	// Number of objects to read concurrently.
	concurrency = 10

	errGetObject   = "error retrieving object from AWS S3"
	errReadEvents  = "error reading events"
	errWriteEvents = "error writing events"
)

// GenerateReport initializes the client code and generates a usage report based on given inputs
func GenerateReport(ctx context.Context, account, endpoint, bucket string, billingPeriod usagetime.Range, w event.Writer) error {
	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		return errors.Wrap(err, "error creating aws session")
	}
	config := &aws.Config{}
	if endpoint != "" {
		config = &aws.Config{
			Endpoint: aws.String(endpoint),
		}
	}
	s3client := s3.New(sess, config)

	if err := maxResourceCountPerGVKPerMCP(ctx, account, bucket, s3client, billingPeriod, w); err != nil {
		return err
	}
	return nil
}

// maxResourceCountPerGVKPerMCP reads usage data for an account and time range
// from bkt and writes aggregated usage events to w. Events are aggregated
// across 1hr windows of the time range.
func maxResourceCountPerGVKPerMCP(ctx context.Context, account, bucket string, client *s3.S3, tr usagetime.Range, w event.Writer) error {
	// TODO: Add support for aggregation windows other than 1 hour.
	iter, err := clientutil.NewUsageQueryIterator(account, tr.Start, tr.End, time.Hour)
	if err != nil {
		return errors.Wrap(err, errReadEvents)
	}

	for iter.More() {
		startPrefix, _, start, end, err := iter.Next()
		if err != nil {
			return errors.Wrap(err, errReadEvents)
		}
		objects, err := client.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(startPrefix),
		})
		if err != nil {
			return errors.Wrap(err, errReadEvents)
		}

		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(concurrency)
		ag := &aggregate.MaxResourceCountPerGVKPerMCP{}
		agMu := &sync.Mutex{}

		for _, obj := range objects.Contents {
			currObject := obj
			g.Go(func() error {
				resp, err := client.GetObjectWithContext(ctx, &s3.GetObjectInput{
					Bucket: aws.String(bucket),
					Key:    currObject.Key,
				})
				if err != nil {
					return errors.Wrap(err, errGetObject)
				}
				return readObject(ag, agMu, resp)
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
func readObject(ag *aggregate.MaxResourceCountPerGVKPerMCP, agMu sync.Locker, obj *s3.GetObjectOutput) error {
	d, err := json.NewMCPGVKEventDecoder(obj.Body)
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
