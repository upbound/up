// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctx

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	itemStyle         = lipgloss.NewStyle()
	kindStyle         = lipgloss.NewStyle().Foreground(neutralColor)
	selectedItemStyle = lipgloss.NewStyle().Foreground(upboundBrandColor)
)

var backNavBinding = key.NewBinding(
	key.WithKeys("left", "h"),
	key.WithHelp("←/h", "back"),
)

var selectNavBinding = key.NewBinding(
	key.WithKeys("right", "enter", "l"),
	key.WithHelp("→/l/enter", "select"),
)

var exitBinding = key.NewBinding(
	key.WithKeys("esc", "ctrl+c"),
	key.WithHelp("esc/ctrl+c", "exit"),
)

var quitBinding = key.NewBinding(
	key.WithKeys("q", "f10"),
	key.WithHelp("q/f10", "switch context & quit"),
)

type KeyFunc func(m model) (model, error)

type padding struct {
	top    int
	bottom int
	left   int
	right  int
}

type item struct {
	text string
	kind string

	onEnter KeyFunc

	padding padding

	matchingTerms []string

	// back denotes that the item will return the user to the previous menu
	back bool

	// notSelectable marks an item as unselectable in the list and will be skipped in navigation
	notSelectable bool
}

func (i item) FilterValue() string { return "" }
func (i item) Matches(s string) bool {
	if strings.EqualFold(s, i.text) {
		return true
	}

	return sets.New(i.matchingTerms...).Has(s)
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	str, ok := listItem.(item)
	if !ok {
		return
	}

	mainStyle := itemStyle
	if index == m.Index() {
		mainStyle = selectedItemStyle
	}
	padding := str.padding
	mainStyle = mainStyle.Copy().Padding(padding.top, padding.right, padding.bottom, padding.left)

	var kind string
	if str.kind != "" {
		kind = fmt.Sprintf("[%s]", str.kind)
	}

	fmt.Fprint(w, lipgloss.JoinHorizontal(lipgloss.Top, // nolint:staticcheck
		kindStyle.Render(fmt.Sprintf("%15s ", kind)),
		mainStyle.Render(str.text),
	))
}

func NewList(items []list.Item) list.Model {
	l := list.New(items, itemDelegate{}, 80, 3)

	l.SetShowTitle(true)
	l.Styles.Title = lipgloss.NewStyle()
	l.SetSpinner(spinner.MiniDot)
	l.SetShowHelp(true)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			backNavBinding,
			selectNavBinding,
		}
	}

	l.KeyMap.ShowFullHelp = key.NewBinding(key.WithDisabled())
	l.KeyMap.CloseFullHelp = key.NewBinding(key.WithDisabled())

	// Try not to start on a back button or unselectable item.
	moveCursorDownUntil(&l, func(cur list.Item) bool {
		i, ok := cur.(item)
		if !ok {
			return true
		}
		return !i.back && !i.notSelectable
	})

	return l
}

func (m model) ListHeight() int {
	lines := 2 // title bar
	for _, i := range m.list.Items() {
		itm := i.(item)
		lines += 1 + strings.Count(itm.text, "\n")
		lines += itm.padding.top + itm.padding.bottom
	}
	lines += 2 // help text

	return lines
}

func (m model) View() string {
	if m.termination != nil {
		return ""
	}

	m.list.Title = m.state.Breadcrumbs()
	l := m.list.View()

	if m.err != nil {
		return fmt.Sprintf("%s\nError: %v", l, m.err)
	}

	return l
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { // nolint:gocyclo // TODO: shorten
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(min(m.windowHeight-4, m.ListHeight()))
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, exitBinding):
			m.termination = &Termination{}
			return m, tea.Quit
		case key.Matches(msg, quitBinding):
			if state, ok := m.state.(Accepting); ok {
				msg, err := state.Accept(m.upCtx, m.navContext)
				if err != nil {
					m.err = err
					return m, nil
				}
				return m.WithTermination(msg, nil), tea.Quit
			}

		case key.Matches(msg, selectNavBinding):
			if i, ok := m.list.SelectedItem().(item); ok {
				// Disable keys and run the spinner while the state updates.
				m.list.KeyMap = list.KeyMap{}
				return m, tea.Sequence(m.list.StartSpinner(), m.updateListState(i.onEnter))
			}

		case key.Matches(msg, backNavBinding):
			if state, ok := m.state.(Back); ok {
				// Disable keys and run the spinner while the state updates.
				m.list.KeyMap = list.KeyMap{}
				return m, tea.Sequence(m.list.StartSpinner(), m.updateListState(state.Back))
			}
		}

	case model:
		m = msg
		m.list.StopSpinner()
		if m.termination != nil {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	m.list = m.moveToSelectableItem(msg)

	return m, cmd
}

func (m model) updateListState(fn KeyFunc) func() tea.Msg {
	return func() tea.Msg {
		newState, err := fn(m)
		if err != nil {
			m.err = err
			return nil
		}
		m = newState

		items, err := m.state.Items(context.Background(), m.upCtx, m.navContext)
		m.err = err

		// recreate the list to reset the cursor position
		if items != nil {
			m.list = NewList(items)
			m.list.SetHeight(min(m.windowHeight-2, m.ListHeight()))
			if _, ok := m.state.(Accepting); ok {
				m.list.KeyMap.Quit = quitBinding
			} else {
				m.list.KeyMap.Quit = key.NewBinding(key.WithDisabled())
			}
		}

		return m
	}
}

func (m model) moveToSelectableItem(msg tea.Msg) list.Model {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m.list
	}

	itemSelectable := func(cur list.Item) bool {
		return !cur.(item).notSelectable
	}

	switch {
	case key.Matches(keyMsg, m.list.KeyMap.CursorUp):
		moveCursorUpUntil(&m.list, itemSelectable)

	case key.Matches(keyMsg, m.list.KeyMap.CursorDown):
		moveCursorDownUntil(&m.list, itemSelectable)
	}

	return m.list
}

type moveCursorConditionFn func(list.Item) bool

// moveCursorDownUntil moves the model's cursor down until cond returns true for
// the selected item. If the bottom of the list is reached without cond
// returning true, the cursor will be moved back up to the last item for which
// cond is true. If no such items exist the first item in the list will be
// selected.
func moveCursorDownUntil(m *list.Model, cond moveCursorConditionFn) {
	moveCursorUntil(m, m.CursorDown, m.CursorUp, cond, true)
}

// moveCursorUpUntil moves the model's cursor up until cond returns true for the
// selected item. If the top of the list is reached without cond returning true,
// the cursor will be moved back down to the first item for which cond is
// true. If no such items exist the last item in the list will be selected.
func moveCursorUpUntil(m *list.Model, cond moveCursorConditionFn) {
	moveCursorUntil(m, m.CursorUp, m.CursorDown, cond, true)
}

// moveCursorUntil uses forward to move the cursor until cond is true. If the
// cursor reaches the first or last item, and bounce is true, backward will be
// used to move back until cond is true or the other end of the list is reached.
//
// Most callers will want to use the wrappers moveCursorDownUntil or
// moveCursorUpUntil.
func moveCursorUntil(m *list.Model, forward, backward func(), cond moveCursorConditionFn, bounce bool) {
	for {
		selected := m.SelectedItem()
		if cond(selected) {
			break
		}

		before := m.Index()
		forward()
		// If we're at the top or bottom, and want to bounce, move the other way.
		if m.Index() == before {
			if bounce {
				moveCursorUntil(m, backward, forward, cond, false)
			}
			break
		}
	}
}
