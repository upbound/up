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
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

type Status struct {
	*tview.Table
	content *statusContent
}

func NewStatus(scrolling Scrolling) *Status {
	s := &Status{
		content: &statusContent{
			scrolling: scrolling,
			progress:  progress.New(progress.WithDefaultGradient()),
		},
	}
	s.content.progress.Width = 12
	s.content.progress.ShowPercentage = false
	s.Table = tview.NewTable().
		SetContent(s.content).
		SetSelectedStyle(tcell.StyleDefault.Bold(true)).
		SetSelectable(true, false)

	return s
}

func (s *Status) SetOffset(offset int) {
	s.Table.SetOffset(offset, 0)
}

func (s *Status) Select(line int) {
	s.Table.Select(line, 0)
}

type statusContent struct {
	scrolling Scrolling
	progress  progress.Model
}

var cell = tview.TableCell{
	Align:           tview.AlignLeft,
	Color:           tcell.ColorDefault,
	BackgroundColor: tcell.ColorDefault,
	Attributes:      tcell.AttrNone,
	Transparent:     true,
}

func (t statusContent) GetCell(row, column int) *tview.TableCell {
	o := t.scrolling.ObjectAt(row - t.scrolling.GetScrollOffset())
	if o == nil {
		cell.Text = ""
		return &cell
	}

	switch column {
	case 0:
		if len(o.Children) > 0 {
			ready := 0
			for _, c := range o.Children {
				if c.JSON.GetCondition(xpv1.TypeReady).Status == corev1.ConditionTrue {
					ready++
				}
			}
			if o.JSON.GetCondition(xpv1.TypeReady).Status == corev1.ConditionTrue {
				ready++
			}

			s := t.progress.ViewAs(float64(ready) / float64(len(o.Children)+1))
			cell.Text = " " + tview.TranslateANSI(s)
		} else {
			cell.Text = strings.Repeat(" ", 13)
		}
	case 1:
		cell.Text = conditionStatus(7, xpv1.TypeSynced, xpkgv1.TypeInstalled)(o)
	case 2:
		cell.Text = conditionStatus(7, xpv1.TypeReady, xpkgv1.TypeHealthy)(o)
	case 3:
		msg := ""
		if cond := o.JSON.GetCondition(xpv1.TypeSynced); cond.Status == corev1.ConditionFalse {
			if cond.Message != "" {
				msg = cond.Message
			} else {
				msg = string(cond.Reason)
			}
		}
		if msg == "" {
			if cond := o.JSON.GetCondition(xpv1.TypeReady); cond.Status == corev1.ConditionFalse {
				if cond.Message != "" {
					msg = cond.Message
				} else {
					msg = string(cond.Reason)
				}
			}
		}
		cell.Text = msg
	default:
		cell.Text = ""
	}
	return &cell
}

func (t statusContent) GetRowCount() int {
	return t.scrolling.GetRowCount()
}

func (t statusContent) GetColumnCount() int {
	return 4
}

func (t statusContent) SetCell(row, column int, cell *tview.TableCell) {
}

func (t statusContent) RemoveRow(row int) {
}

func (t statusContent) RemoveColumn(column int) {
}

func (t statusContent) InsertRow(row int) {
}

func (t statusContent) InsertColumn(column int) {
}

func (t statusContent) Clear() {
	//TODO implement me
	panic("implement me")
}
