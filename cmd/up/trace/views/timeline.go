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
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/upbound/up/cmd/up/trace/model"
	"github.com/upbound/up/cmd/up/trace/style"
)

type TimeLine struct {
	tview.Box

	offset   int
	selected int

	model     *model.App
	scrolling Scrolling
}

func NewTimeLine(scrolling Scrolling, model *model.App) *TimeLine {
	return &TimeLine{
		Box:       *tview.NewBox().SetBorder(false),
		scrolling: scrolling,
		model:     model,
	}
}

func (t *TimeLine) SetOffset(offset int) {
	t.offset = offset
}

func (t *TimeLine) Select(selected int) {
	t.selected = selected
}

func (t *TimeLine) GetSelected() int {
	return t.selected
}

func (t *TimeLine) GetOffset() int {
	return t.offset
}

func (t *TimeLine) Draw(screen tcell.Screen) {
	t.Box.Draw(screen)

	rightTS := t.model.TimeLine.FixedTime
	if rightTS.IsZero() {
		rightTS = time.Now()
	}

	shift := int(int64(rightTS.Truncate(t.model.TimeLine.Scale).Sub(rightTS)) * 10 / int64(t.model.TimeLine.Scale))
	x0, y0, w, h := t.Box.GetInnerRect()

	// draw life-cycle
	for line := 0; line < h; line++ {
		o := t.scrolling.ObjectAt(line)
		if o == nil {
			continue
		}
		y := y0 + line

		s := &style.DefaultTimelineStyle
		if line == t.selected-t.offset {
			s = &style.SelectedTimeLineStyle
		}

		created := t.mapTS(o.CreationTimestamp, x0, w)
		deleted := x0 + w
		if !o.DeletionTimestamp.IsZero() {
			deleted = t.mapTS(o.DeletionTimestamp, x0, w)
		}

		if created > x0+w-1 {
			screen.SetContent(x0+w-1, y, '>', nil, s.Later)
		}
		if deleted < x0 {
			screen.SetContent(x0, y, '<', nil, s.Earlier)
		}

		var x int
		for x = x0; x < created; x++ {
			screen.SetContent(x, y, ' ', nil, s.NotExisting)
		}
		ts := t.mapY(x, x0, w)
		for ; x <= min(deleted-1, x0+w-1); x++ {
			if !o.IsSynced(ts) {
				screen.SetContent(x, y, ' ', nil, s.NotSynced)
			} else if !o.IsReady(ts) {
				screen.SetContent(x, y, ' ', nil, s.NotReady)
			} else {
				screen.SetContent(x, y, ' ', nil, s.Ready)
			}
			ts = ts.Add(t.model.TimeLine.Scale / 10)
		}
		for x := deleted; deleted <= x0+w-1; deleted++ {
			screen.SetContent(x, y, ' ', nil, s.Deleted)
		}
	}

	// draw vertical lines
	for y := y0; y < y0+h; y++ {
		for x := x0 + w - 1; x+shift >= x0; x -= 10 {
			col := tcell.ColorGray
			if y-y0 == t.selected-t.offset {
				col = tcell.ColorWhite
			}
			if c, _, style, _ := screen.GetContent(x+shift, y); c == ' ' {
				screen.SetContent(x+shift, y, '│', nil, style.Foreground(col))
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (t *TimeLine) mapTS(ts time.Time, x0, w int) int {
	rightTS := t.model.TimeLine.FixedTime
	if rightTS.IsZero() {
		rightTS = time.Now()
	}

	return x0 + w - 1 - int(int64(rightTS.Sub(ts))*10/int64(t.model.TimeLine.Scale))
}

func (t *TimeLine) mapY(x, x0, w int) time.Time {
	rightTS := t.model.TimeLine.FixedTime
	if rightTS.IsZero() {
		rightTS = time.Now()
	}

	return rightTS.Add(-time.Duration(x0+w-1-x) * t.model.TimeLine.Scale / 10)
}
