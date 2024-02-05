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
	Tree     Tree
	TimeLine TimeLine
	Error    atomic.Value
	Zoomed   bool

	Kind  atomic.Pointer[string]
	Group atomic.Pointer[string]
	Name  atomic.Pointer[string]
}

func NewApp(kind, group, name string) App {
	a := App{
		Tree: NewTree(),
		TimeLine: TimeLine{
			Scale: DefaultScale,
		},
	}

	a.Error.Store("")

	a.Kind.Store(&kind)
	a.Group.Store(&group)
	a.Name.Store(&name)

	return a
}
