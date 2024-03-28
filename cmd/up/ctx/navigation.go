package ctx

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/upbound"
)

var (
	upboundRootStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	pathSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))
	pathSegmentStyle   = lipgloss.NewStyle()
)

// NavigationState is a model state that provides a list of items for a navigation node.
type NavigationState interface {
	Items(ctx context.Context) ([]list.Item, error)
	Breadcrumbs() string
}

// Accepting is a model state that provides a method to accept a navigation node.
type Accepting interface {
	Accept(ctx context.Context, upCtx *upbound.Context) (string, error)
}

// Back is a model state that provides a method to go back to the parent navigation node.
type Back interface {
	Back(ctx context.Context, upCtx *upbound.Context, m model) (model, error)
}

type AcceptingFunc func(ctx context.Context, upCtx *upbound.Context) error

func (f AcceptingFunc) Accept(ctx context.Context, upCtx *upbound.Context) error {
	return f(ctx, upCtx)
}

// Space provides the navigation node for a space.
type Space struct {
	spaceKubeconfig clientcmd.ClientConfig
}

func (s *Space) Items(ctx context.Context) ([]list.Item, error) {
	config, err := s.spaceKubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}
	nss := &corev1.NamespaceList{}
	if err := cl.List(ctx, nss); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(nss.Items)+1)
	for _, ns := range nss.Items {
		items = append(items, item{text: ns.Name, kind: "group", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
			m.state = &Group{spaceKubeconfig: s.spaceKubeconfig, name: ns.Name}
			return m, nil
		}})
	}

	if len(nss.Items) == 0 {
		items = append(items, item{text: "No groups found"})
	}

	return items, nil
}

func (s *Space) Breadcrumbs() string {
	return upboundRootStyle.Render("Upbound") + pathSegmentStyle.Render(" space")
}

// Group provides the navigation node for a concrete group aka namespace.
type Group struct {
	spaceKubeconfig clientcmd.ClientConfig
	name            string
}

var _ Accepting = &Group{}

func (g *Group) Items(ctx context.Context) ([]list.Item, error) {
	// list controlplanes in group
	config, err := g.spaceKubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}
	ctps := &spacesv1beta1.ControlPlaneList{}
	if err := cl.List(ctx, ctps, client.InNamespace(g.name)); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(ctps.Items)+2)
	items = append(items, item{text: "..", kind: "groups", onEnter: g.Back})

	for _, ctp := range ctps.Items {
		items = append(items, item{text: ctp.Name, kind: "controlplane", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
			m.state = &ControlPlane{NamespacedName: types.NamespacedName{Name: ctp.Name, Namespace: g.name}, spaceKubeconfig: g.spaceKubeconfig}
			return m, nil
		}})
	}
	items = append(items, item{text: "Save as kubectl context", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
		msg, err := g.Accept(ctx, upCtx)
		if err != nil {
			return m, err
		}
		return m.WithTermination(msg, nil), nil
	}, padding: []int{1, 0, 0}})

	if len(ctps.Items) == 0 {
		items = append(items, item{text: "No ControlPlanes found"})
	}

	return items, nil
}

func (g *Group) Breadcrumbs() string {
	return upboundRootStyle.Render("Upbound") + pathSegmentStyle.Render(" space") + pathSeparatorStyle.Render(" > ") + pathSegmentStyle.Render(g.name)
}

func (g *Group) Back(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
	m.state = &Space{spaceKubeconfig: g.spaceKubeconfig}
	return m, nil
}

// ControlPlane provides the navigation node for a concrete controlplane.
type ControlPlane struct {
	spaceKubeconfig clientcmd.ClientConfig
	types.NamespacedName
}

var _ Accepting = &ControlPlane{}

func (ctp *ControlPlane) Items(ctx context.Context) ([]list.Item, error) {
	return []list.Item{
		item{text: "..", kind: "group", onEnter: ctp.Back},
		item{text: fmt.Sprintf("Connect to %s", ctp.NamespacedName), onEnter: KeyFunc(func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
			msg, err := ctp.Accept(ctx, upCtx)
			if err != nil {
				return m, err
			}
			return m.WithTermination(msg, nil), nil
		}), padding: []int{1, 0, 0}},
	}, nil
}

func (ctp *ControlPlane) Breadcrumbs() string {
	return upboundRootStyle.Render("Upbound") + pathSegmentStyle.Render(" space") + pathSeparatorStyle.Render(" > ") + pathSegmentStyle.Render(ctp.Namespace) + pathSeparatorStyle.Render("/") + pathSegmentStyle.Render(ctp.Name)
}

func (ctp *ControlPlane) Back(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
	m.state = &Group{spaceKubeconfig: ctp.spaceKubeconfig, name: ctp.Namespace}
	return m, nil
}
