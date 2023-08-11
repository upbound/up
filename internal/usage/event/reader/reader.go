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

package reader

import (
	"context"
	"errors"

	"github.com/upbound/up/internal/usage/event"
	"github.com/upbound/up/internal/usage/model"
)

var ErrEOF = event.ErrEOF

var _ event.Reader = &MultiReader{}

// MultiReader is the logical concatenation of its readers. They're read
// sequentially. Once all readers have returned EOF, Read will return EOF. If
// any of the readers return a non-nil, non-EOF error, Read will return that
// error. Readers are closed when they return EOF or when Close() is called.
type MultiReader struct {
	Readers []event.Reader
}

func (r *MultiReader) Read(ctx context.Context) (model.MCPGVKEvent, error) {
	for {
		if len(r.Readers) < 1 {
			return model.MCPGVKEvent{}, ErrEOF
		}
		er := r.Readers[0]
		e, err := er.Read(ctx)
		if !errors.Is(err, ErrEOF) {
			return e, err
		}
		if err := er.Close(); err != nil {
			return model.MCPGVKEvent{}, err
		}
		r.Readers = r.Readers[1:]
	}
}

func (r *MultiReader) Close() error {
	for _, er := range r.Readers {
		if err := er.Close(); err != nil {
			return err
		}
	}
	return nil
}
