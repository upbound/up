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
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/upbound/up/cmd/up/trace/model"
	"github.com/upbound/up/cmd/up/trace/style"
)

var _ Scrolling = &Tree{}

type Tree struct {
	*tview.TreeView

	model     *model.Tree
	app       *tview.Application
	scrollers []Scroller
}

type Scrolling interface {
	ObjectAt(line int) *model.Object
	GetCurrentNode() *tview.TreeNode
	GetRowCount() int
	GetScrollOffset() int

	AddScroller(s ...Scroller)
}

type Scroller interface {
	SetOffset(offset int)
	Select(line int)
}

func NewTree(app *tview.Application, m *model.Tree) *Tree {
	t := &Tree{
		TreeView: tview.NewTreeView().
			SetRoot(m.Root()).
			SetCurrentNode(m.Root()).
			SetGraphicsColor(style.TreeGraphics).
			SetTopLevel(1),
		app:   app,
		model: m,
	}
	t.SetSelectedFunc(func(node *tview.TreeNode) {
		node.SetExpanded(!node.IsExpanded())
	})

	return t
}

func (t *Tree) AddScroller(ss ...Scroller) {
	t.scrollers = append(t.scrollers, ss...)

	t.updateScrollers()
}

func (t *Tree) ObjectAt(line int) *model.Object {
	fieldValue := reflect.ValueOf(t.TreeView).Elem().FieldByName("nodes")
	nodes := unsafe.Slice((**tview.TreeNode)(fieldValue.UnsafePointer()), fieldValue.Len())

	n := line + t.GetScrollOffset()
	if n < 0 || n >= len(nodes) {
		return nil
	}
	ref := nodes[n].GetReference()
	if ref == nil {
		return nil
	}
	return ref.(*model.Object)
}

func (t *Tree) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	delegate := t.TreeView.InputHandler()

	return t.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		n := t.TreeView.GetCurrentNode()
		if n == nil {
			return
		}

		defer t.updateScrollers()

		switch {
		case event.Key() == tcell.KeyRight, event.Key() == tcell.KeyRune && event.Rune() == '+':
			if n.IsExpanded() && len(n.GetChildren()) > 0 {
				return
			}
			event = tcell.NewEventKey(tcell.KeyEnter, ' ', tcell.ModNone)
		case event.Key() == tcell.KeyLeft, event.Key() == tcell.KeyRune && event.Rune() == '-':
			if len(n.GetChildren()) == 0 || !n.IsExpanded() {
				if n.GetLevel() <= 1 {
					return
				}

				t.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
					if node == n {
						t.SetCurrentNode(parent)
						parent.Collapse()
						return false
					}
					return true
				})
				return
			}
			n.Collapse()
			return
		case event.Key() == tcell.KeyRune && event.Rune() == 'a':
			t.model.AutoCollapse = !t.model.AutoCollapse
		}

		delegate(event, setFocus)
	})
}

func (t *Tree) updateScrollers() {
	go t.app.QueueUpdateDraw(func() {
		for _, s := range t.scrollers {
			s.SetOffset(t.GetScrollOffset())
		}

		// select the current node in all scrollers
		line := -1 // skip root
		current := t.GetCurrentNode()
		hidden := map[string]bool{}
		t.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
			if node == current {
				for _, s := range t.scrollers {
					s.Select(line)
				}
				return false
			}
			// count only if visible
			if parent == nil || (parent.IsExpanded() && !hidden[fmt.Sprintf("%p", parent)]) {
				line++
			} else {
				hidden[fmt.Sprintf("%p", node)] = true
			}
			return true
		})
	})
}
