// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cleanup

import "errors"

// Cleaner allows adding cleanup funcs to execute in defer.
type Cleaner struct {
	fns []func() error
}

// New creates a new Cleaner.
func New() *Cleaner {
	return &Cleaner{}
}

// Add adds a cleanup func to the Cleaner.
// It does nothing if provided func is nil.
func (c *Cleaner) Add(fn func() error) {
	if fn == nil {
		return
	}
	c.fns = append(c.fns, fn)
}

// OnError runs cleanup fns if the provided error pointer is not nil.
// It executes cleanup fns in reverse order they were added in to
// simulate behavior of stacked defer calls.
// The error pointer will be joined with errors returned from cleanup
// funcs and replaced with this joined error.
func (c *Cleaner) OnError(errr *error) {
	if *errr == nil || len(c.fns) == 0 {
		return
	}
	errs := make([]error, 0, len(c.fns)+1)
	errs = append(errs, *errr)
	// run cleanup fns in reverse order.
	for i := len(c.fns) - 1; i >= 0; i-- {
		if err := c.fns[i](); err != nil {
			errs = append(errs, err)
		}
	}
	*errr = errors.Join(errs...)
}
