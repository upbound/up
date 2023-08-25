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

package azure

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/usage/encoding/json"
	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
)

var ErrEOF = event.ErrEOF

var _ event.Reader = &PagerEventReader{}

// PagerEventReader reads usage events from a pager for blob list responses.
type PagerEventReader struct {
	Client     *container.Client
	Pager      *runtime.Pager[container.ListBlobsFlatResponse]
	currReader *ListBlobsResponseEventReader
}

func (r *PagerEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	for {
		if r.currReader == nil {
			if !r.Pager.More() {
				return model.MXPGVKEvent{}, ErrEOF
			}
			resp, err := r.Pager.NextPage(ctx)
			if err != nil {
				return model.MXPGVKEvent{}, err
			}
			r.currReader = &ListBlobsResponseEventReader{Client: r.Client, Response: &resp}
		}
		e, err := r.currReader.Read(ctx)
		if !errors.Is(err, ErrEOF) {
			return e, err
		}
		r.currReader = nil
	}
}

func (r *PagerEventReader) Close() error {
	if r.currReader == nil {
		return nil
	}
	return r.currReader.Close()
}

var _ event.Reader = &ListBlobsResponseEventReader{}

// ListBlobsResponseEventReader reads usage events from a blob list response.
type ListBlobsResponseEventReader struct {
	Client     *container.Client
	Response   *container.ListBlobsFlatResponse
	itemIdx    int
	currReader *BlobEventReader
}

func (r *ListBlobsResponseEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	for {
		if r.currReader == nil {
			if r.itemIdx >= len(r.Response.Segment.BlobItems) {
				return model.MXPGVKEvent{}, ErrEOF
			}

			blob := r.Response.Segment.BlobItems[r.itemIdx]
			if blob.Name == nil {
				return model.MXPGVKEvent{}, fmt.Errorf("blob name is nil")
			}

			contentType := ""
			if blob.Properties.ContentType != nil {
				contentType = *blob.Properties.ContentType
			}

			r.currReader = &BlobEventReader{
				Client:      r.Client.NewBlobClient(*blob.Name),
				ContentType: contentType,
			}
			r.itemIdx += 1
		}
		if e, err := r.currReader.Read(ctx); !errors.Is(err, ErrEOF) {
			return e, err
		}
		if err := r.currReader.Close(); err != nil {
			return model.MXPGVKEvent{}, err
		}
		r.currReader = nil
	}
}

func (r *ListBlobsResponseEventReader) Close() error {
	if r.currReader == nil {
		return nil
	}
	return r.currReader.Close()
}

var _ event.Reader = &BlobEventReader{}

// BlobEventReader reads usage events from a blob client.
type BlobEventReader struct {
	Client      *blob.Client
	ContentType string
	decoder     *json.MXPGVKEventDecoder
	closers     []io.Closer
}

func (r *BlobEventReader) Read(ctx context.Context) (model.MXPGVKEvent, error) {
	if r.decoder == nil {
		resp, err := r.Client.DownloadStream(ctx, nil)
		if err != nil {
			return model.MXPGVKEvent{}, err
		}

		var body io.ReadCloser
		switch r.ContentType {
		case "application/gzip":
			fallthrough
		case "application/x-gzip":
			r.closers = append(r.closers, resp.Body)
			body, err = gzip.NewReader(resp.Body)
			if err != nil {
				return model.MXPGVKEvent{}, err
			}
		default:
			body = resp.Body
		}
		r.closers = append(r.closers, body)

		decoder, err := json.NewMXPGVKEventDecoder(body)
		if err != nil {
			return model.MXPGVKEvent{}, err
		}
		r.decoder = decoder
	}

	if !r.decoder.More() {
		return model.MXPGVKEvent{}, ErrEOF
	}
	return r.decoder.Decode()
}

func (r *BlobEventReader) Close() error {
	// Close closers in reverse.
	for i := len(r.closers) - 1; i >= 0; i-- {
		if err := r.closers[i].Close(); err != nil {
			return err
		}
	}
	return nil
}
