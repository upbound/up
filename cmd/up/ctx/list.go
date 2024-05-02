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
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/upbound/up/internal/upbound"
)

var (
	itemStyle         = lipgloss.NewStyle()
	kindStyle         = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
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

type KeyFunc func(ctx context.Context, upCtx *upbound.Context, m model) (model, error)

type item struct {
	text string
	kind string

	onEnter KeyFunc

	padding []int

	matchingTerms []string

	// emptyList denotes that the item is marking that the list is empty, and
	// should not be considered an element in the list itself
	emptyList bool

	// back denotes that the item will return the user to the previous menu
	back bool
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
	if len(str.padding) > 0 {
		mainStyle = mainStyle.Copy().Padding(str.padding...)
	}

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

	l.SetShowTitle(false)
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

	// check for initial cursor conditions
	if len(items) > 1 {
		nested := items[0].(item).back
		empty := items[1].(item).emptyList

		if nested && !empty {
			// move the cursor down below the '..' button
			l.CursorDown()
		}

		if nested && empty {
			// disable selecting the empty list item
			l.KeyMap.CursorDown = key.NewBinding(key.WithDisabled())
		}
	}

	return l
}

func (m model) ListHeight() int {
	lines := 0
	for _, i := range m.list.Items() {
		itm := i.(item)
		lines += 1 + strings.Count(itm.text, "\n")
		switch len(itm.padding) {
		case 1, 2:
			lines += itm.padding[0]
		case 3, 4:
			lines += itm.padding[0] + itm.padding[2]
		}
	}
	lines += 2 // help text

	return lines
}

func (m model) View() string {
	if m.termination != nil {
		return ""
	}

	l := m.list.View()
	if m.err != nil {
		return fmt.Sprintf("%s\n\n%s\nError: %v", m.state.Breadcrumbs(), l, m.err)
	}

	return fmt.Sprintf("%s\n\n%s", m.state.Breadcrumbs(), l)
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
				msg, err := state.Accept(context.Background(), m.upCtx, m.contextWriter)
				if err != nil {
					m.err = err
					return m, nil
				}
				return m.WithTermination(msg, nil), tea.Quit
			}

		case key.Matches(msg, selectNavBinding):
			fallthrough
		case key.Matches(msg, backNavBinding):
			var fn KeyFunc
			switch {
			case key.Matches(msg, backNavBinding):
				if state, ok := m.state.(Back); ok {
					if state.CanBack() {
						fn = state.Back
					}
				}
			case key.Matches(msg, selectNavBinding):
				if i, ok := m.list.SelectedItem().(item); ok {
					fn = i.onEnter
				}
			}
			if fn != nil {
				newState, err := fn(context.Background(), m.upCtx, m)
				if err != nil {
					m.err = err
					return m, nil
				}
				m = newState

				items, err := m.state.Items(context.Background(), m.upCtx)
				if err != nil {
					m.err = err
					return m, nil
				}

				// recreate the list to reset the cursor position
				m.list = NewList(items)
				m.list.SetHeight(min(m.windowHeight-2, m.ListHeight()))
				if _, ok := m.state.(Accepting); ok {
					m.list.KeyMap.Quit = quitBinding
				} else {
					m.list.KeyMap.Quit = key.NewBinding(key.WithDisabled())
				}

				if m.termination != nil {
					return m, tea.Quit
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}
