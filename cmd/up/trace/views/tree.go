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
	GetCurrentLine() int

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
	nodes := unsafe.Slice((**tview.TreeNode)(fieldValue.UnsafePointer()), fieldValue.Len()) // nolint:gosec // no way around this

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

func (t *Tree) GetCurrentLine() int {
	// find the current line, from the first visible node
	found := false
	line := -1 // skip root
	current := t.GetCurrentNode()
	t.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		if node == current {
			found = true
			return false
		}

		if !found {
			line++
		}

		return node.IsExpanded() && !found
	})

	return line
}

func (t *Tree) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) { // nolint:gocyclo // TODO: split up
	return t.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) { // nolint:gocyclo // TODO: split up
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

		t.TreeView.InputHandler()(event, setFocus)
	})
}

func (t *Tree) updateScrollers() {
	offset := t.GetScrollOffset()
	line := t.GetCurrentLine()

	for _, s := range t.scrollers {
		s.SetOffset(offset)
		s.Select(line)
	}
}
