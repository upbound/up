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
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/duration"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	xpkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/up/cmd/up/trace/model"
	"github.com/upbound/up/cmd/up/trace/style"
)

type Details struct {
	*tview.Table
	content *detailsContent
}

func NewDetails(scrolling Scrolling) *Details {
	d := &Details{
		Table: tview.NewTable(),
		content: &detailsContent{
			scrolling: scrolling,
			progress:  progress.New(progress.WithGradient(style.DetailsProgressLeft.CSS(), style.DetailsProgressRight.CSS())),
		},
	}
	d.Table.SetContent(d.content)

	return d
}

type detailsContent struct {
	scrolling Scrolling
	progress  progress.Model
}

func conditionFn(fn func(cond *xpv1.Condition) string, conds ...xpv1.ConditionType) func(o *model.Object) string {
	return func(o *model.Object) string {
		for _, name := range conds {
			if cond := o.JSON.GetCondition(name); cond.Status != corev1.ConditionUnknown || cond.Reason != "" || cond.Message != "" {
				return fn(&cond)
			}
		}
		return fn(nil)
	}
}

func shorten(n int, s string) string {
	if n == 0 {
		return s
	}
	if len(s) > n {
		return s[:(n-2)] + ".."
	}
	left := (n - len(s)) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", n-left-len(s))
}

func conditionStatus(n int, conds ...xpv1.ConditionType) func(o *model.Object) string {
	return conditionFn(func(cond *xpv1.Condition) string {
		if cond == nil {
			return "[#626262]" + shorten(n, "n/a")
		}
		switch cond.Status {
		case corev1.ConditionTrue:
			return "[#17e1cf]" + shorten(n, "True")
		case corev1.ConditionFalse:
			return "[violet]" + shorten(n, "False")
		case corev1.ConditionUnknown:
			return "[#626262]" + shorten(n, "Unknown")
		default:
			return "[#626262]" + shorten(n, "n/a")
		}
	}, conds...)
}

func conditionMessage(conds ...xpv1.ConditionType) func(o *model.Object) string {
	return conditionFn(func(cond *xpv1.Condition) string {
		if cond == nil {
			return ""
		}
		if cond.Message == "" {
			return string(cond.Reason)
		}
		return fmt.Sprintf("%s (%s)", cond.Message, cond.Reason)
	}, conds...)
}

func conditionType(conds ...xpv1.ConditionType) func(o *model.Object) string {
	return conditionFn(func(cond *xpv1.Condition) string {
		if cond == nil {
			return "[darkgrey]" + string(conds[0])
		}
		return "[darkgrey]" + string(cond.Type)
	}, conds...)
}

func minLen(n int, s string) string {
	if len(s) > n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

var detailCells = map[int]map[int]any{
	0: {
		0: "Kind", 1: func(o *model.Object) string {
			if o.Group == "" {
				return o.Kind
			} else {
				return fmt.Sprintf("%s [darkgrey]Group[-] %s", o.Kind, o.Group)
			}
		},
		2: func(o *model.Object) string {
			return minLen(7, conditionType(xpv1.TypeSynced, xpkgv1.TypeInstalled)(o))
		}, 3: conditionStatus(7, xpv1.TypeSynced, xpkgv1.TypeInstalled), 6: conditionMessage(xpv1.TypeSynced, xpkgv1.TypeInstalled),
	},
	1: {
		0: "Object", 1: func(o *model.Object) string { return objectName(o.Namespace, o.Name) },
		2: conditionType(xpv1.TypeReady, xpkgv1.TypeHealthy), 3: conditionStatus(7, xpv1.TypeReady, xpkgv1.TypeHealthy), 6: conditionMessage(xpv1.TypeReady, xpkgv1.TypeHealthy),
	},
	2: {
		0: "ControlPlane", 1: func(o *model.Object) string {
			return minLen(60, fmt.Sprintf("%s/%s", o.ControlPlane.Namespace, o.ControlPlane.Name))
		},
	},
	3: {
		0: "Created", 1: func(o *model.Object) string {
			cr := duration.HumanDuration(time.Since(o.CreationTimestamp))
			if !o.DeletionTimestamp.IsZero() {
				return fmt.Sprintf("%s ago   [darkgrey]Deleted[-] %s [darkgrey]ago", cr, duration.HumanDuration(time.Since(o.DeletionTimestamp)))
			}
			return fmt.Sprintf("%s [darkgrey]ago", cr)
		},
	},
}

func objectName(ns, name string) string {
	if ns == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", ns, name)
}

func (t detailsContent) GetCell(row, column int) *tview.TableCell {
	cur := t.scrolling.GetCurrentNode()
	if cur == nil {
		return &tview.TableCell{}
	}
	ref := cur.GetReference()
	if ref == nil {
		return &tview.TableCell{}
	}
	o := ref.(*model.Object)

	switch c := detailCells[row][column].(type) {
	case string:
		return &tview.TableCell{
			Align:           tview.AlignRight,
			Color:           style.DetailsKeyFg,
			BackgroundColor: style.DetailsKeyBg,
			Attributes:      tcell.AttrNone,
			Transparent:     true,
			Text:            c,
		}
	case tview.TableCell:
		return &c
	case func(o *model.Object) string:
		return &tview.TableCell{
			Align:           tview.AlignLeft,
			Color:           style.DetailsValueFg,
			BackgroundColor: style.DetailsValueBg,
			Attributes:      tcell.AttrNone,
			Transparent:     true,
			Text:            c(o),
		}
	case func(o *model.Object) *tview.TableCell:
		return c(o)
	default:
		return &tview.TableCell{}
	}
}

func (t detailsContent) GetRowCount() int {
	return 4
}

func (t detailsContent) GetColumnCount() int {
	return 8
}

func (t detailsContent) SetCell(row, column int, cell *tview.TableCell) {
}

func (t detailsContent) RemoveRow(row int) {
}

func (t detailsContent) RemoveColumn(column int) {
}

func (t detailsContent) InsertRow(row int) {
}

func (t detailsContent) InsertColumn(column int) {
}

func (t detailsContent) Clear() {
	//TODO implement me
	panic("implement me")
}
