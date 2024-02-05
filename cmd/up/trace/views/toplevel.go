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
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"k8s.io/apimachinery/pkg/util/duration"

	"github.com/upbound/up/cmd/up/trace/model"
	"github.com/upbound/up/cmd/up/trace/style"
)

type TopLevel struct {
	*tview.Grid

	model          *model.App
	inputHandlerFn func(event *tcell.EventKey, setFocus func(p tview.Primitive))

	titles    []gridTitle
	subTitles []gridTitle
}

func NewTopLevel(title string, model *model.App, grid *tview.Grid, inputHandlerFn func(event *tcell.EventKey, setFocus func(p tview.Primitive))) *TopLevel {
	tl := &TopLevel{
		Grid:           grid,
		model:          model,
		inputHandlerFn: inputHandlerFn,
	}

	tl.titles = []gridTitle{
		{col: 0, row: 0, text: fmt.Sprintf(" [::b]%s ", title), color: tcell.GetColor("#af7efd"), align: tview.AlignCenter},
		{col: 1, row: 1, fn: func(screen tcell.Screen, x0, y, w int) {
			ts := tl.model.TimeLine.Scale
			if !tl.model.TimeLine.FixedTime.IsZero() {
				ts += time.Since(tl.model.TimeLine.FixedTime.Truncate(tl.model.TimeLine.Scale / 10))
			}
			for x := x0 + w - 14; x > x0-4; x -= 10 {
				tview.Print(screen, duration.HumanDuration(ts), x, y, 9, tview.AlignCenter, tcell.ColorDarkGray)
				ts += tl.model.TimeLine.Scale
			}
			if tl.model.TimeLine.FixedTime.IsZero() {
				tview.Print(screen, "Now", x0+w-2, y, 3, tview.AlignLeft, tcell.ColorDarkGray)
			} else {
				screen.SetContent(x0+w, y, '>', nil, tcell.StyleDefault.Foreground(tcell.ColorDarkGray))
			}
		}},
		{col: 2, row: 1, text: "── Progress ── Synced Ready  Message ", color: tcell.ColorDarkGray, align: tview.AlignLeft},
		{col: 0, row: 2, text: " Details ", color: tcell.ColorDarkGray, align: tview.AlignCenter},
		{col: 0, row: 2, fn: func(screen tcell.Screen, x, y, w int) {
			var b strings.Builder
			if tl.model.Tree.AutoCollapse {
				b.WriteString("AutoCollapse ")
			}
			if tl.model.Zoomed {
				b.WriteString("Zoomed ")
			}

			if b.Len() > 0 {
				tview.Print(screen, " [::b]"+b.String(), x, y, w, tview.AlignRight, tcell.ColorYellow)
			}
		}},
	}
	tl.subTitles = []gridTitle{
		{col: 0, row: 2, fn: func(screen tcell.Screen, x, y, w int) {
			err := tl.model.Error.Load().(string)
			tview.Print(screen, err, x, y, w, tview.AlignCenter, tcell.ColorHotPink)
		}},
	}

	return tl
}

type gridTitle struct {
	col, row int

	text string
	fn   func(screen tcell.Screen, x, y, w int)

	color tcell.Color
	align int
}

func (t *TopLevel) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return t.inputHandlerFn
}

func (p *TopLevel) Draw(screen tcell.Screen) {
	p.Grid.Draw(screen)

	// collect grid items
	fv := reflect.ValueOf(p.Grid).Elem().FieldByName("items")
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
	for _, title := range p.titles {
		item, ok := prims[int64(title.row)][int64(title.col)]
		if !ok {
			continue
		}
		x := item.FieldByName("x").Int()
		y := item.FieldByName("y").Int() - 1 // -1 to draw above cell
		w := item.FieldByName("w").Int()

		if title.text != "" {
			tview.Print(screen, title.text, int(x), int(y), int(w), title.align, title.color)
		}
		if title.fn != nil {
			title.fn(screen, int(x), int(y), int(w))
		}
	}

	// draw subtitles at the bottom
	for _, title := range p.subTitles {
		item, ok := prims[int64(title.row)][int64(title.col)]
		if !ok {
			continue
		}
		x := item.FieldByName("x").Int()
		y := item.FieldByName("y").Int() + 1 // +1 to draw below the cell
		w := item.FieldByName("w").Int()
		h := item.FieldByName("h").Int()

		if title.text != "" {
			tview.Print(screen, title.text, int(x), int(y)+int(h)-1, int(w), title.align, title.color)
		}
		if title.fn != nil {
			title.fn(screen, int(x), int(y)+int(h)-1, int(w))
		}
	}

	// draw F1-F10 hints
	cmds := []string{"Help", "Kind", "View", "Edit", "", "", "", "", "", "Quit"}
	w, h := screen.Size()
	for x := 0; x < w; x++ {
		screen.SetCell(x, h-1, tcell.StyleDefault.Background(style.BottomKeys), ' ')
	}
	for i := 0; i < 10; i++ {
		text := fmt.Sprintf("[lightgray::]F%d[-] %s", i+1, cmds[i])
		tview.Print(screen, text, w/10*i, h-1, w/10, tview.AlignLeft, tcell.ColorWhite)
	}
}
