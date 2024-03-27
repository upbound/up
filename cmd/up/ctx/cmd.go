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

	"github.com/alecthomas/kong"
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

type model struct {
	windowHeight int
	list         list.Model

	state State
	err   error

	termination *Termination
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(min(m.windowHeight-2, len(m.list.Items())))
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c", "f10":
			m.termination = &Termination{}
			return m, tea.Quit

		case "enter":
			if i, ok := m.list.SelectedItem().(item); ok && i.action != nil {
				newState, term, err := i.action.Exec(context.Background(), m.state)
				m.err = err
				if term != nil {
					m.termination = term
					return m, tea.Quit
				}
				if newState != nil {
					m.state = newState
					items, err := m.state.Items(context.Background())
					if err != nil {
						m.err = err
					}
					m.list.SetItems(items)
					m.list.SetHeight(min(m.windowHeight-2, len(m.list.Items())))
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.termination != nil {
		return m.termination.Message
	}

	l := m.list.View()
	if m.err != nil {
		return fmt.Sprintf("%s\nError: %v", l, m.err)
	}
	return l
}

func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	ctrl.SetLogger(zap.New(zap.WriteTo(io.Discard)))

	kubeconfig, err := upCtx.Profile.GetSpaceKubeConfig()
	if err != nil {
		return err
	}

	initial := &Space{
		spaceKubeconfig: kubeconfig,
		base: base{
			upCtx: upCtx,
		},
	}
	items, err := initial.Items(ctx)
	if err != nil {
		return err
	}

	m := model{
		list:  NewList(items),
		state: initial,
	}
	result, err := tea.NewProgram(m).Run()
	if m := result.(model); m.termination != nil {
		return m.termination.Err
	}
	return nil
}
