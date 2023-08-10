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
	"compress/gzip"
	"context"
	"errors"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
)

var EOF = event.EOF

var _ event.Reader = &QueryEventReader{}

type QueryEventReader struct {
	Bucket *storage.BucketHandle
	Query  *storage.Query
	reader *ObjectIteratorEventReader
}

func (r *QueryEventReader) Read(ctx context.Context) (model.MCPGVKEvent, error) {
	if r.reader == nil {
		r.reader = &ObjectIteratorEventReader{Bucket: r.Bucket, Iterator: r.Bucket.Objects(ctx, r.Query)}
	}
	return r.reader.Read(ctx)
}

func (r *QueryEventReader) Close() error {
	if r.reader == nil {
		return nil
	}
	return r.reader.Close()
}

var _ event.Reader = &ObjectIteratorEventReader{}

type ObjectIteratorEventReader struct {
	Bucket     *storage.BucketHandle
	Iterator   *storage.ObjectIterator
	currReader *ObjectHandleEventReader
}

func (r *ObjectIteratorEventReader) Read(ctx context.Context) (model.MCPGVKEvent, error) {
	for {
		if r.currReader == nil {
			attrs, err := r.Iterator.Next()
			if errors.Is(err, iterator.Done) {
				return model.MCPGVKEvent{}, EOF
			}
			r.currReader = &ObjectHandleEventReader{Object: r.Bucket.Object(attrs.Name), Attrs: attrs}
		}
		if e, err := r.currReader.Read(ctx); !errors.Is(err, EOF) {
			return e, err
		}
		if err := r.currReader.Close(); err != nil {
			return model.MCPGVKEvent{}, err
		}
		r.currReader = nil
	}
}

func (r *ObjectIteratorEventReader) Close() error {
	if r.currReader == nil {
		return nil
	}
	return r.currReader.Close()
}

var _ event.Reader = &ObjectHandleEventReader{}

type ObjectHandleEventReader struct {
	Object  *storage.ObjectHandle
	Attrs   *storage.ObjectAttrs
	decoder *json.MCPGVKEventDecoder
	closers []io.Closer
}

func (r *ObjectHandleEventReader) Read(ctx context.Context) (model.MCPGVKEvent, error) {
	if r.decoder == nil {
		reader, err := r.Object.NewReader(ctx)
		if err != nil {
			return model.MCPGVKEvent{}, err
		}

		contentType := ""
		if r.Attrs != nil {
			contentType = r.Attrs.ContentType
		}

		var body io.ReadCloser
		switch contentType {
		case "application/gzip":
			fallthrough
		case "application/x-gzip":
			r.closers = append(r.closers, reader)
			body, err = gzip.NewReader(reader)
			if err != nil {
				return model.MCPGVKEvent{}, err
			}
		default:
			body = reader
		}
		r.closers = append(r.closers, body)

		decoder, err := json.NewMCPGVKEventDecoder(body)
		if err != nil {
			return model.MCPGVKEvent{}, err
		}
		r.decoder = decoder
	}
	if !r.decoder.More() {
		return model.MCPGVKEvent{}, EOF
	}
	return r.decoder.Decode()
}

func (r *ObjectHandleEventReader) Close() error {
	// Close closers in reverse.
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i].Close(); err != nil {
			return err
		}
	}
	return nil
}
