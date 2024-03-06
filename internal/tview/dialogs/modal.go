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
	"reflect"
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func ShowModal(app *tview.Application, p tview.Primitive) *Modal {
	m := &Modal{
		Primitive:  p,
		app:        app,
		background: GetRoot(app),
	}
	app.SetRoot(m, true)
	return m
}

func GetRoot(app *tview.Application) tview.Primitive {
	fld := reflect.ValueOf(app).Elem().FieldByName("root")
	return *(*tview.Primitive)(unsafe.Pointer(fld.UnsafeAddr()))
}

type Modal struct {
	tview.Primitive

	app        *tview.Application
	background tview.Primitive
}

func (m *Modal) Draw(screen tcell.Screen) {
	m.background.Draw(screen)
	m.Primitive.Draw(screen)
}

func (m *Modal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	delegate := m.Primitive.InputHandler()
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if event.Key() == tcell.KeyEsc {
			m.app.SetRoot(m.background, true)
		}
		delegate(event, setFocus)
	}
}

func (m *Modal) Hide() {
	if GetRoot(m.app) == m.Primitive {
		m.app.SetRoot(m.background, true)
	}
}
