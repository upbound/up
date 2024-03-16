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
	"sync/atomic"
	"time"

	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/internal/tview/model"
)

const DefaultScale = time.Second * 10

var Scales = []time.Duration{
	time.Second * 10,
	time.Second * 30,
	time.Minute,
	time.Minute * 5,
	time.Minute * 15,
	time.Minute * 30,
}

type App struct {
	TopLevel model.TopLevel
	Tree     Tree
	TimeLine TimeLine
	Zoomed   bool

	Resources      atomic.Pointer[[]string]
	GroupKindNames atomic.Pointer[query.GroupKindNames]
	CategoryNames  atomic.Pointer[query.CategoryNames]
}

func NewApp(resources []string, gkns query.GroupKindNames, cns query.CategoryNames) *App {
	a := &App{
		Tree: NewTree(),
		TimeLine: TimeLine{
			Scale: DefaultScale,
		},
	}

	a.Resources.Store(&resources)
	a.GroupKindNames.Store(&gkns)
	a.CategoryNames.Store(&cns)

	return a
}
