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

package dialogs

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func Modal(app *tview.Application, background, m tview.Primitive) {
	app.SetRoot(&modal{
		Primitive:  m,
		app:        app,
		background: background,
	}, true)
}

type modal struct {
	tview.Primitive

	app        *tview.Application
	background tview.Primitive

	cancelFn  func()
	buttonsFn []func()
}

func (m *modal) Draw(screen tcell.Screen) {
	m.background.Draw(screen)
	m.Primitive.Draw(screen)
}

func (m *modal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	delegate := m.Primitive.InputHandler()
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if event.Key() == tcell.KeyEsc {
			m.app.SetRoot(m.background, true)
		}
		delegate(event, setFocus)
	}
}
