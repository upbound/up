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

package undo

import (
	"errors"
	"sync"
)

// Undoer allows registring undo functions.
type Undoer interface {
	// Undo registers an undo function.
	Undo(fn func() error)
}

// Do runs a func with a new Undoer.
// If the func returns error, all the undo funcs, registered with the Undoer
// will run in reverse order and all errors will be joined with the main error.
func Do(fn func(u Undoer) error) error {
	u := &tx{}
	err := fn(u)
	if err == nil {
		return nil
	}
	fns := u.steps()
	if len(fns) == 0 {
		return err
	}
	errs := make([]error, 0, len(fns)+1)
	errs = append(errs, err)
	// run cleanup fns in reverse order.
	for i := len(fns) - 1; i >= 0; i-- {
		if err := fns[i](); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

var _ Undoer = (*tx)(nil)

type tx struct {
	mu   sync.Mutex
	done bool
	cfns []func() error
}

// Undo implements Undoer.
func (t *tx) Undo(fn func() error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		panic("Do func completed")
	}
	t.cfns = append(t.cfns, fn)
}

func (t *tx) steps() []func() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.done = true
	return t.cfns
}
