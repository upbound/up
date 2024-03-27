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
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

const (
	contextSwitchedFmt = "Kubeconfig context %q switched to: %s\n"
)

func init() {
	runtime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

type Cmd struct {
	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`

	Argument    string `arg:"" optional:"" help:".. to move to the parent, '-' for the previous context, '.' for the current context, or any relative path."`
	Short       bool   `short:"s" env:"UP_SHORT" name:"short" help:"Short output."`
	KubeContext string `env:"UP_CONTEXT" default:"upbound" name:"context" help:"Kubernetes context to operate on."`
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

	upCtx       *upbound.Context
	kubeContext string
}

func (m model) WithTermination(msg string, err error) model {
	m.termination = &Termination{Message: msg, Err: err}
	return m
}

func (c *Cmd) Run(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context) error {
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
	case "-":
		return c.RunSwap(ctx, upCtx)
	case "":
		return c.RunInteractive(ctx, kongCtx, upCtx, initialState)
	default:
		return c.RunRelative(ctx, kongCtx, upCtx, initialState)
	}
}

func (c *Cmd) RunDot(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context, initialState NavigationState) error {
	if _, ok := initialState.(*Profiles); ok {
		po := clientcmd.NewDefaultPathOptions()
		conf, err := po.GetStartingConfig()
		if err != nil {
			return err
		}
		fmt.Fprintf(kongCtx.Stderr, "Non-%s kubeconfig config: %s\n", upboundRootStyle.Render("Upbound"), conf.CurrentContext)
		return nil
	}

	if c.Short {
		switch state := initialState.(type) {
		case *Group:
			fmt.Printf("%s/%s\n", state.space.profile, state.name)
		case *ControlPlane:
			fmt.Printf("%s/%s/%s\n", state.space.profile, state.NamespacedName.Namespace, state.NamespacedName.Name)
		}
	} else {
		fmt.Printf("Current kubeconfig context %q: %s\n", c.KubeContext, initialState.Breadcrumbs())
	}
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

	if last != c.KubeContext+upboundPreviousContextSuffix {
		// switch to last context trivially via CurrentContext
		oldCurrent := conf.CurrentContext
		conf.CurrentContext = last
		if err := clientcmd.ModifyConfig(po, *conf, true); err != nil {
			return err
		}
		if err := writeLastContext(oldCurrent); err != nil {
			return err
		}
		fmt.Printf(contextSwitchedFmt, c.KubeContext, last)
		return nil
	}

	// swap upbound and upbound-previous context
	prev, ok := conf.Contexts[c.KubeContext+upboundPreviousContextSuffix]
	if !ok {
		return fmt.Errorf("no %q context found", c.KubeContext+upboundPreviousContextSuffix)
	}
	current, ok := conf.Contexts[c.KubeContext]
	if !ok {
		return fmt.Errorf("no %q context found", c.KubeContext)
	}
	conf.Contexts[c.KubeContext] = prev
	conf.Contexts[c.KubeContext+upboundPreviousContextSuffix] = current

	// swap upbound and upbound-previous cluster
	if conf.Contexts[c.KubeContext].Cluster == c.KubeContext+upboundPreviousContextSuffix {
		prev := conf.Clusters[c.KubeContext+upboundPreviousContextSuffix]
		if prev == nil {
			return fmt.Errorf("no %q cluster found", c.KubeContext+upboundPreviousContextSuffix)
		}
		conf.Clusters[c.KubeContext] = prev
		if current := conf.Clusters[c.KubeContext]; current == nil {
			delete(conf.Clusters, c.KubeContext+upboundPreviousContextSuffix)
		} else {
			conf.Clusters[c.KubeContext+upboundPreviousContextSuffix] = current
		}
		if !ok {
			delete(conf.Clusters, c.KubeContext+upboundPreviousContextSuffix)
		}
		for _, ctx := range conf.Contexts {
			if ctx.Cluster == c.KubeContext+upboundPreviousContextSuffix {
				ctx.Cluster = c.KubeContext
			}
			if ctx.Cluster == c.KubeContext {
				ctx.Cluster = c.KubeContext + upboundPreviousContextSuffix
			}
		}
	}

	// swap upbound and upbound-previous authInfo
	if conf.Contexts[c.KubeContext].AuthInfo == c.KubeContext+upboundPreviousContextSuffix {
		prev := conf.AuthInfos[c.KubeContext+upboundPreviousContextSuffix]
		if prev == nil {
			return fmt.Errorf("no %q authInfo found", c.KubeContext+upboundPreviousContextSuffix)
		}
		conf.AuthInfos[c.KubeContext] = prev
		if current := conf.AuthInfos[c.KubeContext]; current == nil {
			delete(conf.AuthInfos, c.KubeContext+upboundPreviousContextSuffix)
		} else {
			conf.AuthInfos[c.KubeContext+upboundPreviousContextSuffix] = current
		}
		if !ok {
			delete(conf.AuthInfos, c.KubeContext+upboundPreviousContextSuffix)
		}
		for _, ctx := range conf.Contexts {
			if ctx.AuthInfo == c.KubeContext+upboundPreviousContextSuffix {
				ctx.AuthInfo = c.KubeContext
			}
			if ctx.AuthInfo == c.KubeContext {
				ctx.AuthInfo = c.KubeContext + upboundPreviousContextSuffix
			}
		}
	}

	// write kubeconfig
	state, err := DeriveState(ctx, upCtx, conf)
	if err != nil {
		return err
	}
	if err := clientcmd.ModifyConfig(po, *conf, true); err != nil {
		return err
	}
	if err := writeLastContext(c.KubeContext + upboundPreviousContextSuffix); err != nil {
		return err
	}
	fmt.Printf(contextSwitchedFmt, c.KubeContext, state.Breadcrumbs())
	return nil
}

func (c *Cmd) RunRelative(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context, initialState NavigationState) error { // nolint:gocyclo // a bit long but ¯\_(ツ)_/¯
	m := model{
		state:       initialState,
		upCtx:       upCtx,
		kubeContext: c.KubeContext,
	}
	for _, s := range strings.Split(c.Argument, "/") {
		switch s {
		case ".":
		case "..":
			back, ok := m.state.(Back)
			if !ok {
				return fmt.Errorf("cannot move to parent context from: %s", m.state.Breadcrumbs())
			}
			var err error
			m, err = back.Back(ctx, upCtx, m)
			if err != nil {
				return err
			}
		default:
			// find the string as item
			items, err := m.state.Items(ctx, upCtx)
			if err != nil {
				return err
			}
			found := false
			for _, i := range items {
				if i, ok := i.(item); ok && i.text == s {
					if i.onEnter == nil {
						return fmt.Errorf("cannot enter %q in: %s", s, m.state.Breadcrumbs())
					}
					m, err = i.onEnter(ctx, m.upCtx, m)
					if err != nil {
						return err
					}
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%q not found in: %s", s, m.state.Breadcrumbs())
			}
		}
	}

	// final step if we moved: access the state
	msg := fmt.Sprintf("Kubeconfig context %q: %s\n", c.KubeContext, m.state.Breadcrumbs())
	if m.state.Breadcrumbs() != initialState.Breadcrumbs() {
		accepting, ok := m.state.(Accepting)
		if !ok {
			return fmt.Errorf("cannot move context to: %s", m.state.Breadcrumbs())
		}
		var err error
		msg, err = accepting.Accept(ctx, upCtx, c.KubeContext)
		if err != nil {
			return err
		}
	}

	if c.Short {
		switch state := m.state.(type) {
		case *Group:
			fmt.Printf("%s/%s\n", state.space.profile, state.name)
		case *ControlPlane:
			fmt.Printf("%s/%s/%s\n", state.space.profile, state.NamespacedName.Namespace, state.NamespacedName.Name)
		}
	} else {
		fmt.Print(msg)
	}

	return nil
}

func (c *Cmd) RunInteractive(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context, initialState NavigationState) error {
	// start interactive mode
	m := model{
		state:       initialState,
		upCtx:       upCtx,
		kubeContext: c.KubeContext,
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
			fmt.Fprint(kongCtx.Stderr, m.termination.Message)
		}
		return m.termination.Err
	}
	return nil
}
