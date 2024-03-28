package ctx

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/upbound/up/internal/upbound"
)

var (
	itemStyle         = lipgloss.NewStyle()
	kindStyle         = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
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
}

func (i item) FilterValue() string { return "" }

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

	fmt.Fprintf(w, lipgloss.JoinHorizontal(lipgloss.Top,
		kindStyle.Render(fmt.Sprintf("%14s ", kind)),
		mainStyle.Render(str.text),
	))
}

func NewList(items []list.Item) list.Model {
	l := list.New(items, itemDelegate{}, 80, 3)
	//l.Title = "What do you want for dinner?"
	l.SetShowTitle(false)
	l.SetShowHelp(true)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)

	l.KeyMap.ShowFullHelp = key.NewBinding(key.WithDisabled())

	return l
}
