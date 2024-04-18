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
	"sort"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up/internal/upbound"
)

var (
	upboundRootStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	pathSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))
	pathSegmentStyle   = lipgloss.NewStyle()
)

// NavigationState is a model state that provides a list of items for a navigation node.
type NavigationState interface {
	Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error)
	Breadcrumbs() string
}

// Accepting is a model state that provides a method to accept a navigation node.
type Accepting interface {
	NavigationState
	Accept(ctx context.Context, upCtx *upbound.Context, kubeContext string) (string, error)
}

// Back is a model state that provides a method to go back to the parent navigation node.
type Back interface {
	NavigationState
	Back(ctx context.Context, upCtx *upbound.Context, m model) (model, error)
}

type AcceptingFunc func(ctx context.Context, upCtx *upbound.Context) error

func (f AcceptingFunc) Accept(ctx context.Context, upCtx *upbound.Context) error {
	return f(ctx, upCtx)
}

type Profiles struct{}

func (p *Profiles) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) {
	if upCtx.Profile.IsSpace() {
		// build list of local profile spaces
		profiles, err := upCtx.Cfg.GetUpboundProfiles()
		if err != nil {
			return nil, err
		}
		items := make([]list.Item, 0, len(profiles))
		for name, p := range profiles {
			if !p.IsSpace() {
				continue
			}
			items = append(items, item{text: name, kind: "space", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
				m.state = &Space{name: name, profile: name}
				return m, nil
			}})
		}
		sort.Sort(sortedItems(items))
		return items, nil
	}

	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return nil, err
	}

	ac := accounts.NewClient(cfg)

	a, err := upbound.GetAccount(ctx, ac, upCtx.Account)
	if err != nil {
		return nil, err
	}

	cloudCfg, err := upCtx.BuildControllerClientConfig()
	if err != nil {
		return nil, err
	}

	cloudClient, err := client.New(cloudCfg, client.Options{})
	if err != nil {
		return nil, err
	}

	var l upboundv1alpha1.SpaceList
	err = cloudClient.List(ctx, &l, &client.ListOptions{Namespace: a.Organization.Name})
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, 0)
	for _, space := range l.Items {
		if space.Spec.Mode == upboundv1alpha1.ModeLegacy {
			continue
		}

		// todo(redbackthomson): Add support for connected spaces
		if space.Spec.Mode == upboundv1alpha1.ModeConnected {
			continue
		}

		items = append(items, item{text: space.GetObjectMeta().GetName(), kind: "space", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
			m.state = &Space{name: space.GetObjectMeta().GetName(), profile: upCtx.ProfileName, cloud: true}
			return m, nil
		}})
	}
	sort.Sort(sortedItems(items))
	return items, nil
}

func (p *Profiles) Breadcrumbs() string {
	return upboundRootStyle.Render("Upbound") + " profiles"
}

type sortedItems []list.Item

func (s sortedItems) Len() int           { return len(s) }
func (s sortedItems) Less(i, j int) bool { return s[i].(item).text < s[j].(item).text }
func (s sortedItems) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

var _ Back = &Space{}

// Space provides the navigation node for a space.
type Space struct {
	profile string
	name    string
	cloud   bool
}

func (s *Space) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) {
	var cl client.Client

	if !s.cloud {
		p, err := upCtx.Cfg.GetUpboundProfile(s.profile)
		if err != nil {
			return nil, err
		}
		config, _, err := p.GetSpaceRestConfig()
		if err != nil {
			return nil, err
		}
		cl, err = client.New(config, client.Options{})
		if err != nil {
			return nil, err
		}
	} else {
		rest, err := upCtx.BuildCloudSpaceRestConfig(s.name, s.profile)
		if err != nil {
			return nil, err
		}

		cl, err = client.New(rest, client.Options{})
		if err != nil {
			return nil, err
		}
	}

	nss := &corev1.NamespaceList{}
	if err := cl.List(ctx, nss, client.MatchingLabels(map[string]string{spacesv1beta1.ControlPlaneGroupLabelKey: "true"})); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(nss.Items)+1)
	items = append(items, item{text: "..", kind: "profiles", onEnter: s.Back, back: true})
	for _, ns := range nss.Items {
		items = append(items, item{text: ns.Name, kind: "group", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
			m.state = &Group{space: *s, name: ns.Name}
			return m, nil
		}})
	}

	if len(nss.Items) == 0 {
		items = append(items, item{text: "No groups found", emptyList: true})
	}

	return items, nil
}

func (s *Space) Back(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
	m.state = &Profiles{}
	return m, nil
}

func (s *Space) Breadcrumbs() string {
	if s.cloud {
		return upboundRootStyle.Render("Upbound") + pathSegmentStyle.Render(" space ") + pathSegmentStyle.Render(s.name)
	}
	return upboundRootStyle.Render("Upbound") + pathSegmentStyle.Render(" profile ") + pathSegmentStyle.Render(s.profile)
}

// Group provides the navigation node for a concrete group aka namespace.
type Group struct {
	space Space
	name  string
}

var _ Accepting = &Group{}
var _ Back = &Group{}

func (g *Group) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) {
	var cl client.Client
	if !g.space.cloud {
		p, err := upCtx.Cfg.GetUpboundProfile(g.space.profile)
		if err != nil {
			return nil, err
		}
		config, _, err := p.GetSpaceRestConfig()
		if err != nil {
			return nil, err
		}
		cl, err = client.New(config, client.Options{})
		if err != nil {
			return nil, err
		}
	} else {
		rest, err := upCtx.BuildCloudSpaceRestConfig(g.space.name, g.space.profile)
		if err != nil {
			return nil, err
		}

		cl, err = client.New(rest, client.Options{})
		if err != nil {
			return nil, err
		}
	}

	ctps := &spacesv1beta1.ControlPlaneList{}
	if err := cl.List(ctx, ctps, client.InNamespace(g.name)); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(ctps.Items)+2)
	items = append(items, item{text: "..", kind: "groups", onEnter: g.Back, back: true})

	for _, ctp := range ctps.Items {
		items = append(items, item{text: ctp.Name, kind: "ctp", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
			m.state = &ControlPlane{group: *g, name: ctp.Name}
			return m, nil
		}})
	}
	/*
		items = append(items, item{text: "Save as kubectl context", onEnter: func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
			msg, err := g.Accept(ctx, upCtx)
			if err != nil {
				return m, err
			}
			return m.WithTermination(msg, nil), nil
		}, padding: []int{1, 0, 0}})
	*/

	if len(ctps.Items) == 0 {
		items = append(items, item{text: "No ControlPlanes found", emptyList: true})
	}

	return items, nil
}

func (g *Group) Breadcrumbs() string {
	return g.space.Breadcrumbs() + pathSeparatorStyle.Render(" > ") + pathSegmentStyle.Render(g.name)
}

func (g *Group) Back(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
	m.state = &g.space
	return m, nil
}

// ControlPlane provides the navigation node for a concrete controlplane.
type ControlPlane struct {
	group Group
	name  string
}

var _ Accepting = &ControlPlane{}
var _ Back = &ControlPlane{}

func (ctp *ControlPlane) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) {
	return []list.Item{
		item{text: "..", kind: "ctps", onEnter: ctp.Back, back: true},
		/*
			item{text: fmt.Sprintf("Connect to %s", ctp.NamespacedName), onEnter: KeyFunc(func(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
				msg, err := ctp.Accept(ctx, upCtx)
				if err != nil {
					return m, err
				}
				return m.WithTermination(msg, nil), nil
			}), padding: []int{1, 0, 0}},
		*/
	}, nil
}

func (ctp *ControlPlane) Breadcrumbs() string {
	return ctp.group.space.Breadcrumbs() + pathSeparatorStyle.Render(" > ") + pathSegmentStyle.Render(ctp.group.name) + pathSeparatorStyle.Render("/") + pathSegmentStyle.Render(ctp.name)
}

func (ctp *ControlPlane) Back(ctx context.Context, upCtx *upbound.Context, m model) (model, error) {
	m.state = &ctp.group
	return m, nil
}

func (ctp *ControlPlane) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: ctp.name, Namespace: ctp.group.name}
}
