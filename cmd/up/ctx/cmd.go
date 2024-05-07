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
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

const (
	contextSwitchedFmt = "Kubeconfig context %q switched to: %s\n"
)

var (
	errParseSpaceContext = errors.New("unable to parse space info from context")
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
	File        string `short:"f" name:"kubeconfig" help:"Kubeconfig to modify when saving a new context"`
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

	upCtx         *upbound.Context
	contextWriter kubeContextWriter
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
	initialState, err := DeriveState(ctx, upCtx, conf, profile.GetIngressHost)
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
		return c.RunRelative(ctx, upCtx, initialState)
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
	state, err := DeriveState(ctx, upCtx, conf, profile.GetIngressHost)
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
	// todo(redbackthomson): Handle the case of no current context
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

func (c *Cmd) RunRelative(ctx context.Context, upCtx *upbound.Context, initialState NavigationState) error { // nolint:gocyclo // a bit long but ¯\_(ツ)_/¯
	// begin from root unless we're starting from a relative . or ..
	state := initialState
	if !strings.HasPrefix(c.Argument, ".") {
		state = &Root{}
	}

	m := model{
		state:         state,
		upCtx:         upCtx,
		contextWriter: c.kubeContextWriter(upCtx),
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
			m, err = back.Back(m)
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
				if i, ok := i.(item); ok && i.Matches(s) {
					if i.onEnter == nil {
						return fmt.Errorf("cannot enter %q in: %s", s, m.state.Breadcrumbs())
					}
					m, err = i.onEnter(m)
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

	// final step if we moved: accept the state
	msg := fmt.Sprintf("Kubeconfig context %q: %s\n", c.KubeContext, m.state.Breadcrumbs())
	if m.state.Breadcrumbs() != initialState.Breadcrumbs() {
		accepting, ok := m.state.(Accepting)
		if !ok {
			return fmt.Errorf("cannot move context to: %s", m.state.Breadcrumbs())
		}
		var err error
		msg, err = accepting.Accept(m.upCtx, m.contextWriter)
		if err != nil {
			return err
		}
	}

	// don't print anything else or we are going to pollute stdout
	if c.File != "-" {
		if c.Short {
			switch state := m.state.(type) {
			case *Group:
				fmt.Printf("%s/%s\n", state.Space.Name, state.Name)
			case *ControlPlane:
				fmt.Printf("%s/%s/%s\n", state.Group.Space.Name, state.NamespacedName().Namespace, state.NamespacedName().Name)
			}
		} else {
			fmt.Print(msg)
		}
	}

	return nil
}

func (c *Cmd) RunInteractive(ctx context.Context, kongCtx *kong.Context, upCtx *upbound.Context, initialState NavigationState) error {
	// start interactive mode
	m := model{
		state:         initialState,
		upCtx:         upCtx,
		contextWriter: c.kubeContextWriter(upCtx),
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

func (c *Cmd) kubeContextWriter(upCtx *upbound.Context) kubeContextWriter {
	if c.File == "-" {
		return &printWriter{}
	}

	return &fileWriter{
		upCtx:            upCtx,
		fileOverride:     c.File,
		kubeContext:      c.KubeContext,
		verify:           kube.VerifyKubeConfig(upCtx.WrapTransport),
		writeLastContext: writeLastContext,
		modifyConfig:     clientcmd.ModifyConfig,
	}
}

type getIngressHostFn func(ctx context.Context, cl client.Client) (host string, ca []byte, err error)

// DeriveState returns the navigation state based on the current context set in
// the given kubeconfig
func DeriveState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config, getIngressHost getIngressHostFn) (NavigationState, error) {
	currentCtx := conf.Contexts[conf.CurrentContext]

	var spaceExt *SpaceExtension
	if conf.CurrentContext == "" || currentCtx == nil {
		return DeriveNewState(ctx, conf, getIngressHost)
	} else if ext, ok := currentCtx.Extensions[ContextExtensionKeySpace]; !ok {
		return DeriveNewState(ctx, conf, getIngressHost)
	} else if spaceExt, ok = ext.(*SpaceExtension); !ok {
		return nil, errors.New("unable to parse space extension to go struct")
	}

	if spaceExt.Spec.Cloud != nil {
		return DeriveExistingCloudState(upCtx, conf, spaceExt.Spec.Cloud)
	} else if spaceExt.Spec.Disconnected != nil {
		return DeriveExistingDisconnectedState(ctx, upCtx, conf, spaceExt.Spec.Disconnected, getIngressHost)
	}
	return nil, errors.New("unable to derive state using context extension")
}

// DeriveNewState derives the current navigation state assuming that the current
// context was created by a process other than the CLI.
// Depending on what we are pointing at, there are a few options as to what to
// do. If spaces **is not** installed in the cluster, then we fall back to root
// Cloud navigation. If spaces **is** installed cluster, we should derive the
// space information from the cluster. For all other cases and for all errors,
// we should fall back to root Cloud navigation.
// TODO(redbackthomson): Add support for passing a non-blocking error message
// back if derivation was partially successful (maybe only when --debug is set?)
func DeriveNewState(ctx context.Context, conf *clientcmdapi.Config, getIngressHost getIngressHostFn) (NavigationState, error) {
	// if no current context, or current is pointing at an invalid context
	if conf.CurrentContext == "" {
		return &Root{}, nil
	} else if _, exists := conf.Contexts[conf.CurrentContext]; !exists {
		return &Root{}, nil
	}

	rest, err := clientcmd.NewDefaultClientConfig(*conf, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return &Root{}, nil // nolint:nilerr
	}

	cl, err := client.New(rest, client.Options{})
	if err != nil {
		return &Root{}, nil // nolint:nilerr
	}

	ingress, ca, err := getIngressHost(ctx, cl)
	if err != nil {
		// ingress inaccessible or doesn't exist
		return &Root{}, nil // nolint:nilerr
	}

	return &Space{
		Name:    conf.CurrentContext,
		Ingress: ingress,
		CA:      ca,

		HubContext: conf.CurrentContext,
	}, nil
}

// DeriveExistingDisconnectedState derives the navigation state assuming the
// current context in the passed kubeconfig is pointing at an existing
// disconnected space created by the CLI
func DeriveExistingDisconnectedState(ctx context.Context, upCtx *upbound.Context, conf *clientcmdapi.Config, disconnected *DisconnectedConfiguration, getIngressHost getIngressHostFn) (NavigationState, error) {
	if _, ok := conf.Contexts[disconnected.HubContext]; !ok {
		return nil, fmt.Errorf("cannot find space hub context %q", disconnected.HubContext)
	}

	var ingress string
	var ctp types.NamespacedName
	var ca []byte

	// determine the ingress either by looking up the base URL if we're in a
	// ctp, or querying for the config map if we're in a group

	ingress, ctp, _ = upCtx.GetCurrentSpaceContextScope()
	if ctp.Name != "" {
		// we're in a ctp, so re-use the CA of the current cluster
		ca = conf.Clusters[conf.Contexts[conf.CurrentContext].Cluster].CertificateAuthorityData
	} else {
		// get ingress from hub
		rest, err := clientcmd.NewDefaultClientConfig(*conf, &clientcmd.ConfigOverrides{
			CurrentContext: disconnected.HubContext,
		}).ClientConfig()
		if err != nil {
			return &Root{}, nil // nolint:nilerr
		}

		cl, err := client.New(rest, client.Options{})
		if err != nil {
			return &Root{}, nil // nolint:nilerr
		}

		ingress, ca, err = getIngressHost(ctx, cl)
		if err != nil {
			// ingress inaccessible or doesn't exist
			return &Root{}, nil // nolint:nilerr
		}
	}

	space := Space{
		Name:    disconnected.HubContext,
		Ingress: ingress,
		CA:      ca,

		HubContext: disconnected.HubContext,
	}

	// derive navigation state
	switch {
	case ctp.Namespace != "" && ctp.Name != "":
		return &ControlPlane{
			Group: Group{
				Space: space,
				Name:  ctp.Namespace,
			},
			Name: ctp.Name,
		}, nil
	case ctp.Namespace != "":
		return &Group{
			Space: space,
			Name:  ctp.Namespace,
		}, nil
	default:
		return &space, nil
	}
}

// DeriveExistingCloudState derives the navigation state assuming that the
// current context in the passed kubeconfig is pointing at an existing Cloud
// space previously created by the CLI
func DeriveExistingCloudState(upCtx *upbound.Context, conf *clientcmdapi.Config, cloud *CloudConfiguration) (NavigationState, error) {
	auth := conf.AuthInfos[conf.Contexts[conf.CurrentContext].AuthInfo]
	ca := conf.Clusters[conf.Contexts[conf.CurrentContext].Cluster].CertificateAuthorityData

	// the exec was modified or wasn't produced by up
	if cloud == nil || cloud.Organization == "" {
		return &Root{}, nil // nolint:nilerr
	}

	org := &Organization{
		Name: cloud.Organization,
	}

	ingress, ctp, exists := upCtx.GetCurrentSpaceContextScope()
	if !exists {
		return nil, errParseSpaceContext
	}

	spaceName := strings.TrimPrefix(strings.Split(ingress, ".")[0], "https://")
	space := Space{
		Org:  *org,
		Name: spaceName,

		Ingress:  strings.TrimPrefix(ingress, "https://"),
		CA:       ca,
		AuthInfo: auth,
	}

	// derive navigation state
	switch {
	case ctp.Namespace != "" && ctp.Name != "":
		return &ControlPlane{
			Group: Group{
				Space: space,
				Name:  ctp.Namespace,
			},
			Name: ctp.Name,
		}, nil
	case ctp.Namespace != "":
		return &Group{
			Space: space,
			Name:  ctp.Namespace,
		}, nil
	default:
		return &space, nil
	}
}
