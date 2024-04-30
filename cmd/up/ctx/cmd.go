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
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/golang-jwt/jwt"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/profile"
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

func (c *Cmd) RunSwap(ctx context.Context, upCtx *upbound.Context) error { // nolint:gocyclo // TODO: shorten
	last, err := readLastContext()
	if err != nil {
		return err
	}

	// load kubeconfig
	confRaw, err := upCtx.Kubecfg.RawConfig()
	if err != nil {
		return err
	}
	conf := &confRaw

	// more complicated case: last context is upbound-previous and we have to rename
	conf, oldContext, err := activateContext(conf, last, c.KubeContext)
	if err != nil {
		return err
	}

	// write kubeconfig
	state, err := DeriveState(ctx, upCtx, conf)
	if err != nil {
		return err
	}
	if err := clientcmd.ModifyConfig(upCtx.Kubecfg.ConfigAccess(), *conf, true); err != nil {
		return err
	}
	if err := writeLastContext(oldContext); err != nil {
		return err
	}
	fmt.Printf(contextSwitchedFmt, c.KubeContext, state.Breadcrumbs())
	return nil
}

func activateContext(conf *clientcmdapi.Config, sourceContext, preferredContext string) (newConf *clientcmdapi.Config, newLastContext string, err error) { // nolint:gocyclo // little long, but well tested
	// switch to non-upbound last context trivially via CurrentContext e.g.
	// - upbound <-> other
	// - something <-> other
	if sourceContext != preferredContext+upboundPreviousContextSuffix {
		oldCurrent := conf.CurrentContext
		conf.CurrentContext = sourceContext
		return conf, oldCurrent, nil
	}

	if sourceContext == conf.CurrentContext {
		return nil, conf.CurrentContext, nil
	}

	// swap upbound and upbound-previous context
	source, ok := conf.Contexts[sourceContext]
	if !ok {
		return nil, "", fmt.Errorf("no %q context found", preferredContext+upboundPreviousContextSuffix)
	}
	current, ok := conf.Contexts[conf.CurrentContext]
	if !ok {
		return nil, "", fmt.Errorf("no %q context found", conf.CurrentContext)
	}
	if conf.CurrentContext == preferredContext {
		conf.Contexts[preferredContext] = source
		conf.Contexts[preferredContext+upboundPreviousContextSuffix] = current
		newLastContext = preferredContext + upboundPreviousContextSuffix
	} else {
		// For other <-> upbound-previous, keep "other" for last context
		conf.Contexts[preferredContext] = source
		delete(conf.Contexts, preferredContext+upboundPreviousContextSuffix)
		newLastContext = conf.CurrentContext
	}
	conf.CurrentContext = preferredContext

	// swap upbound and upbound-previous cluster
	if conf.Contexts[preferredContext].Cluster == preferredContext+upboundPreviousContextSuffix {
		prev := conf.Clusters[preferredContext+upboundPreviousContextSuffix]
		if prev == nil {
			return nil, "", fmt.Errorf("no %q cluster found", preferredContext+upboundPreviousContextSuffix)
		}
		if current := conf.Clusters[preferredContext]; current == nil {
			delete(conf.Clusters, preferredContext+upboundPreviousContextSuffix)
		} else {
			conf.Clusters[preferredContext+upboundPreviousContextSuffix] = current
		}
		conf.Clusters[preferredContext] = prev
		for _, ctx := range conf.Contexts {
			if ctx.Cluster == preferredContext+upboundPreviousContextSuffix {
				ctx.Cluster = preferredContext
			} else if ctx.Cluster == preferredContext {
				ctx.Cluster = preferredContext + upboundPreviousContextSuffix
			}
		}
	}

	// swap upbound and upbound-previous authInfo
	if conf.Contexts[preferredContext].AuthInfo == preferredContext+upboundPreviousContextSuffix {
		prev := conf.AuthInfos[preferredContext+upboundPreviousContextSuffix]
		if prev == nil {
			return nil, "", fmt.Errorf("no %q authInfo found", preferredContext+upboundPreviousContextSuffix)
		}
		if current := conf.AuthInfos[preferredContext]; current == nil {
			delete(conf.AuthInfos, preferredContext+upboundPreviousContextSuffix)
		} else {
			conf.AuthInfos[preferredContext+upboundPreviousContextSuffix] = current
		}
		conf.AuthInfos[preferredContext] = prev
		for _, ctx := range conf.Contexts {
			if ctx.AuthInfo == preferredContext+upboundPreviousContextSuffix {
				ctx.AuthInfo = preferredContext
			} else if ctx.AuthInfo == preferredContext {
				ctx.AuthInfo = preferredContext + upboundPreviousContextSuffix
			}
		}
	}

	return conf, newLastContext, nil
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
			fmt.Printf("%s/%s\n", state.space.name, state.name)
		case *ControlPlane:
			fmt.Printf("%s/%s/%s\n", state.group.space.name, state.NamespacedName().Namespace, state.NamespacedName().Name)
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

func DeriveState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config) (NavigationState, error) {
	ingress, ctp, exists := upCtx.ParseCurrentSpaceContextURL()

	var spaceKubeconfig *clientcmdapi.Config
	var err error

	// if we're already pointing at the space level
	if !exists {
		spaceKubeconfig = conf
	} else {
		// reference the space server
		config, err := upCtx.Kubecfg.RawConfig()
		if err != nil {
			return nil, err
		}

		config = *config.DeepCopy()
		config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server = ingress
		spaceKubeconfig = &config
	}

	rest, err := clientcmd.NewDefaultClientConfig(*spaceKubeconfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return &Root{}, nil
	}

	spaceClient, err := client.New(rest, client.Options{})
	if err != nil {
		return &Root{}, nil
	}

	// determine if self-hosted by looking for ingress
	host, ca, err := profile.GetIngressHost(ctx, spaceClient)
	if meta.IsNoMatchError(err) || kerrors.IsUnauthorized(err) {
		return DeriveCloudState(ctx, upCtx, conf)
	} else if err != nil {
		return nil, err
	}

	return DeriveSelfHostedState(ctx, upCtx, conf, host, ca, ctp)
}

func DeriveSelfHostedState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config, ingress string, ca []byte, ctp types.NamespacedName) (NavigationState, error) {
	auth := conf.AuthInfos[conf.Contexts[conf.CurrentContext].AuthInfo]

	space := Space{
		name:     conf.CurrentContext,
		ingress:  ingress,
		ca:       ca,
		authInfo: auth,
	}

	// derive navigation state
	switch {
	case ctp.Namespace != "" && ctp.Name != "":
		return &ControlPlane{
			group: Group{
				space: space,
				name:  ctp.Namespace,
			},
			name: ctp.Name,
		}, nil
	case ctp.Namespace != "":
		return &Group{
			space: space,
			name:  ctp.Namespace,
		}, nil
	default:
		return &space, nil
	}
}

func DeriveCloudState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config) (NavigationState, error) {
	auth := conf.AuthInfos[conf.Contexts[conf.CurrentContext].AuthInfo]

	// not authenticated with an Upbound JWT, start from empty
	if auth == nil {
		return &Root{}, nil
	}

	token, err := ParseJWT(auth.Token)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, err
	}

	orgClaim, ok := claims["organization"]
	if !ok {
		return &Root{}, nil
	}

	org := &Organization{
		name: orgClaim.(string),
	}

	ingress, ctp, exists := upCtx.ParseCurrentSpaceContextURL()
	if !exists {
		return org, nil
	}

	spaceName := strings.TrimPrefix(strings.Split(ingress, ".")[0], "https://")
	space := Space{
		org:  *org,
		name: spaceName,

		ingress:  strings.TrimPrefix(ingress, "https://"),
		ca:       make([]byte, 0),
		authInfo: auth,
	}

	// derive navigation state
	switch {
	case ctp.Namespace != "" && ctp.Name != "":
		return &ControlPlane{
			group: Group{
				space: space,
				name:  ctp.Namespace,
			},
			name: ctp.Name,
		}, nil
	case ctp.Namespace != "":
		return &Group{
			space: space,
			name:  ctp.Namespace,
		}, nil
	default:
		return &space, nil
	}
}

func ParseJWT(encoded string) (*jwt.Token, error) {
	token, _, error := new(jwt.Parser).ParseUnverified(encoded, jwt.MapClaims{})
	return token, error
}
