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

package ctx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

func init() {
	runtime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

type Cmd struct {
	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	if !upCtx.Profile.IsSpace() {
		// TODO: add legacy support
		return errors.New("Only Spaces contexts supported for now.")
	}
	return nil
}

// Termination is a model state that indicates the command should be terminated,
// optionally with a message and an error.
type Termination struct {
	Err     error
	Message string
}

type model struct {
	windowHeight int
	list         list.Model

	state NavigationState
	err   error

	termination *Termination

	upCtx *upbound.Context
}

func (m model) WithTermination(msg string, err error) model {
	m.termination = &Termination{Message: msg, Err: err}
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(min(m.windowHeight-4, m.ListHeight()))
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			m.termination = &Termination{}
			return m, tea.Quit

		case "q", "f10":
			if state, ok := m.state.(Accepting); ok {
				msg, err := state.Accept(context.Background(), m.upCtx)
				if err != nil {
					m.err = err
					return m, nil
				}
				return m.WithTermination(msg, nil), tea.Quit
			}

		case "enter", "left":
			var fn KeyFunc
			switch keypress {
			case "left":
				if state, ok := m.state.(Back); ok {
					fn = state.Back
				}
			case "enter":
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

				items, err := m.state.Items(context.Background())
				if err != nil {
					m.err = err
					return m, nil
				}

				m.list.SetItems(items)
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

func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	ctrl.SetLogger(zap.New(zap.WriteTo(io.Discard)))

	kubeconfig, err := upCtx.Profile.GetSpaceKubeConfig()
	if err != nil {
		return err
	}

	m := model{
		state: &Space{
			spaceKubeconfig: kubeconfig,
		},
		upCtx: upCtx,
	}

	items, err := m.state.Items(ctx)
	if err != nil {
		return err
	}
	m.list = NewList(items)
	m.list.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	if _, ok := m.state.(Accepting); ok {
		m.list.KeyMap.Quit = quitBinding
	}

	result, err := tea.NewProgram(m).Run()
	if m := result.(model); m.termination != nil {
		if m.termination.Message != "" {
			fmt.Println(m.termination.Message)
		}
		return m.termination.Err
	}
	return nil
}
