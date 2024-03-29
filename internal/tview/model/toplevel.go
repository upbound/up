// Copyright 2024 Upbound Inc
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

package model

import (
	"sync"
	"time"
)

const errorShowDuration = 3 * time.Second

type TopLevel struct {
	lock  sync.RWMutex
	err   error
	errTS time.Time
}

func (t *TopLevel) SetError(err error) {
	if err == nil {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()
	t.err = err
	t.errTS = time.Now()
}
func (t *TopLevel) Error() error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if time.Since(t.errTS) > errorShowDuration {
		return nil
	}
	return t.err
}
