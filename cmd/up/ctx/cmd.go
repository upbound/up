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
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

const (
	contextSwitchedFmt = "Kubeconfig context \"upbound\" switched to: %s\n"
)

func init() {
	runtime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

type Cmd struct {
	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`

	Argument string `arg:"" optional:"" help:".. to move to the parent and '-' to move to the previous context."`
	Short    bool   `short:"s" env:"UP_SHORT" name:"short" help:"Short output."`
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

func (c *Cmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	ctrl.SetLogger(zap.New(zap.WriteTo(io.Discard)))

	// find profile and derive controlplane from kubeconfig
	po := clientcmd.NewDefaultPathOptions()
	conf, err := po.GetStartingConfig()
	if err != nil {
		return err
	}
	initialState, err := DeriveState(ctx, upCtx, conf)
	if err != nil {
		return err
	}

	// non-interactive mode via positional argument
	switch c.Argument {
	case "..":
		return c.RunDotDot(ctx, upCtx, initialState)
	case "-":
		return c.RunSwap(ctx, upCtx)
	case ".":
		return c.RunDot(ctx, upCtx, initialState)
	case "":
		return c.RunInteractive(ctx, upCtx, initialState)
	default:
		return fmt.Errorf("invalid argument: %s", c.Argument)
	}
}

func (c *Cmd) RunDot(ctx context.Context, upCtx *upbound.Context, initialState NavigationState) error {
	if _, ok := initialState.(*Profiles); ok {
		po := clientcmd.NewDefaultPathOptions()
		conf, err := po.GetStartingConfig()
		if err != nil {
			return err
		}
		fmt.Printf("Non-%s kubeconfig config: %s\n", upboundRootStyle.Render("Upbound"), conf.CurrentContext)
		return nil
	}

	if c.Short {
		switch state := initialState.(type) {
		case *Group:
			fmt.Printf("upbound/%s/%s\n", state.space.profile, state.name)
		case *ControlPlane:
			fmt.Printf("upbound/%s/%s/%s\n", state.space.profile, state.NamespacedName.Namespace, state.NamespacedName.Name)
		}
	} else {
		fmt.Printf("Current kubeconfig context: %s\n", initialState.Breadcrumbs())
	}
	return nil
}

func (c *Cmd) RunDotDot(ctx context.Context, upCtx *upbound.Context, state NavigationState) error {
	back, ok := state.(Back)
	if !ok {
		return fmt.Errorf("cannot move to parent context from: %s", state.Breadcrumbs())
	}
	m, err := back.Back(ctx, upCtx, model{state: state, upCtx: upCtx})
	if err != nil {
		return err
	}
	/*
		if space, ok := m.state.(*Space); ok {
			// skipping a level like the space level directly to profiles
			m, err = space.Back(context.Background(), upCtx, m)
			if err != nil {
				return err
			}
		}
	*/
	accepting, ok := m.state.(Accepting)
	if !ok {
		// skipping a level like the group level
		return fmt.Errorf("cannot move to parent context from: %s", state.Breadcrumbs())
	}
	msg, err := accepting.Accept(ctx, upCtx)
	if err != nil {
		return err
	}
	fmt.Print(msg)
	return nil
}

func (c *Cmd) RunSwap(ctx context.Context, upCtx *upbound.Context) error { // nolint:gocyclo // TODO: shorten
	last, err := readLastContext()
	if err != nil {
		return err
	}

	// load kubeconfig
	po := clientcmd.NewDefaultPathOptions()
	conf, err := po.GetStartingConfig()
	if err != nil {
		return err
	}

	if last != upboundPreviousContext {
		// switch to last context trivially via CurrentContext
		oldCurrent := conf.CurrentContext
		conf.CurrentContext = last
		if err := clientcmd.ModifyConfig(po, *conf, true); err != nil {
			return err
		}
		if err := writeLastContext(oldCurrent); err != nil {
			return err
		}
		fmt.Printf(contextSwitchedFmt, last)
		return nil
	}

	// switch upbund and upbound-previous
	prev, ok := conf.Contexts[upboundPreviousContext]
	if !ok {
		return fmt.Errorf("no previous context found")
	}
	upbound, ok := conf.Contexts[upboundContext]
	if !ok {
		return fmt.Errorf("no upbound context found")
	}
	conf.Contexts[upboundContext] = prev
	conf.Contexts[upboundPreviousContext] = upbound
	state, err := DeriveState(ctx, upCtx, conf)
	if err != nil {
		return err
	}
	if err := clientcmd.ModifyConfig(po, *conf, true); err != nil {
		return err
	}
	if err := writeLastContext(upboundPreviousContext); err != nil {
		return err
	}
	fmt.Printf(contextSwitchedFmt, state.Breadcrumbs())
	return nil
}

func (c *Cmd) RunInteractive(ctx context.Context, upCtx *upbound.Context, initialState NavigationState) error {
	// start interactive mode
	m := model{
		state: initialState,
		upCtx: upCtx,
	}
	items, err := m.state.Items(ctx, upCtx)
	if err != nil {
		return err
	}
	m.list = NewList(items)
	m.list.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	if _, ok := m.state.(Accepting); ok {
		m.list.KeyMap.Quit = quitBinding
	}

	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	if m, ok := result.(model); !ok {
		return fmt.Errorf("unexpected model type: %T", result)
	} else if m.termination != nil {
		if m.termination.Message != "" {
			fmt.Print(m.termination.Message)
		}
		return m.termination.Err
	}
	return nil
}
