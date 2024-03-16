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

package views

import (
	"fmt"
	"reflect"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/upbound/up/internal/tview/dialogs"
	"github.com/upbound/up/internal/tview/style"
)

type TopLevel struct {
	*tview.Grid
	app *tview.Application

	Titles    []GridTitle
	SubTitles []GridTitle
	Commands  []string
	Error     func() error

	delegate func(event *tcell.EventKey, setFocus func(p tview.Primitive)) bool

	escPending bool // next digit turns key into F1-F10
}

func NewTopLevel(title string, grid *tview.Grid, app *tview.Application) *TopLevel {
	tl := &TopLevel{
		Grid: grid,
		app:  app,
		Titles: []GridTitle{
			{Col: 0, Row: 0, Text: fmt.Sprintf(" [::b]%s ", title), Color: tcell.GetColor("#af7efd"), Align: tview.AlignCenter},
		},
		Commands: []string{"", "", "", "", "", "", "", "", "", "Quit"},
	}

	return tl
}

func (t *TopLevel) SetTitles(titles ...GridTitle) *TopLevel {
	t.Titles = append(titles[:1], t.Titles...)
	return t
}

func (t *TopLevel) SetSubTitles(titles ...GridTitle) *TopLevel {
	t.SubTitles = titles
	return t
}

func (t *TopLevel) SetCommands(commands ...string) *TopLevel {
	t.Commands = commands
	return t
}

func (t *TopLevel) SetError(f func() error) *TopLevel {
	t.Error = f
	return t
}

func (t *TopLevel) SetDelegateInputHandler(h func(event *tcell.EventKey, setFocus func(p tview.Primitive)) bool) *TopLevel {
	t.delegate = h
	return t
}

type GridTitle struct {
	Col, Row int

	Text string
	Fn   func(screen tcell.Screen, x, y, w int)

	Color tcell.Color
	Align int
}

func (t *TopLevel) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) { // nolint:gocyclo // input handlers don't get easier to read when split.
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if t.escPending && event.Modifiers() == tcell.ModNone {
			// turn esc-0 => F10, esc-1 => F1, etc.
			switch event.Rune() {
			case '0':
				event = tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone)
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				event = tcell.NewEventKey(tcell.KeyF1+tcell.Key(event.Rune()-'1'), 0, tcell.ModNone)
			}
		}
		t.escPending = false

		if t.delegate != nil {
			if processed := t.delegate(event, setFocus); processed {
				return
			}
		}

		switch event.Key() { // nolint:exhaustive // there is a default case
		case tcell.KeyEscape:
			t.escPending = true
		case tcell.KeyF10:
			t.app.Stop()
		default:
		}

		if f := t.app.GetFocus(); f != nil && f != t {
			f.InputHandler()(event, setFocus)
		}
	}
}

func (t *TopLevel) InteractiveQuit() {
	oldRoot := dialogs.GetRoot(t.app)
	dialogs.ShowModal(t.app, dialogs.NewConfirmDialog().
		SetTitle("Quit").
		SetText("Do you want to quit?").
		SetSelectedFunc(t.app.Stop).
		SetCancelFunc(func() { t.app.SetRoot(oldRoot, true) }).
		Display())
}

func (t *TopLevel) Draw(screen tcell.Screen) { // nolint:gocyclo // draw methods are always long, splitting does not make them easier
	t.Grid.Draw(screen)

	// collect grid items
	fv := reflect.ValueOf(t.Grid).Elem().FieldByName("items")
	prims := map[int64]map[int64]reflect.Value{}
	for i := 0; i < fv.Len(); i++ {
		item := fv.Index(i).Elem()
		row := item.FieldByName("Row").Int()
		col := item.FieldByName("Column").Int()
		if _, ok := prims[row]; !ok {
			prims[row] = map[int64]reflect.Value{}
		}
		prims[row][col] = item
	}

	// draw titles on top of grid cells
	for _, title := range t.Titles {
		item, ok := prims[int64(title.Row)][int64(title.Col)]
		if !ok {
			continue
		}
		x := item.FieldByName("x").Int()
		y := item.FieldByName("y").Int() - 1 // -1 to draw above cell
		w := item.FieldByName("w").Int()

		if title.Text != "" {
			tview.Print(screen, title.Text, int(x), int(y), int(w), title.Align, title.Color)
		}
		if title.Fn != nil {
			title.Fn(screen, int(x), int(y), int(w))
		}
	}

	// draw subtitles at the bottom
	for _, title := range t.SubTitles {
		item, ok := prims[int64(title.Row)][int64(title.Col)]
		if !ok {
			continue
		}
		x := item.FieldByName("x").Int()
		y := item.FieldByName("y").Int() + 1 // +1 to draw below the cell
		w := item.FieldByName("w").Int()
		h := item.FieldByName("h").Int()

		if title.Text != "" {
			tview.Print(screen, title.Text, int(x), int(y)+int(h)-1, int(w), title.Align, title.Color)
		}
		if title.Fn != nil {
			title.Fn(screen, int(x), int(y)+int(h)-1, int(w))
		}
	}

	// draw error and keep it up at least for errorHideInterval
	var err error
	if t.Error != nil {
		err = t.Error()
	}
	if err != nil {
		w, h := screen.Size()
		for x := 0; x < w; x++ {
			screen.SetCell(x, h-1, tcell.StyleDefault.Background(style.ErrorBarBackground), ' ')
		}
		tview.Print(screen, err.Error(), 0, h-1, w, tview.AlignCenter, style.ErrorBarForeground)
		return
	}

	// draw F1-F10 hints
	w, h := screen.Size()
	for x := 0; x < w; x++ {
		screen.SetCell(x, h-1, tcell.StyleDefault.Background(style.BottomKeys), ' ')
	}
	for i := 0; i < 10; i++ {
		text := fmt.Sprintf("[lightgray::]F%d[-] %s", i+1, t.Commands[i])
		tview.Print(screen, text, w/10*i, h-1, w/10, tview.AlignLeft, tcell.ColorWhite)
	}
}
