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
	"os"
	"os/exec"
	"sort"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up-sdk-go/service/organizations"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/version"
)

var (
	upboundBrandColor = lipgloss.AdaptiveColor{Light: "#5e3ba5", Dark: "#af7efd"}
	neutralColor      = lipgloss.AdaptiveColor{Light: "#4e5165", Dark: "#9a9ca7"}
)

var (
	// all adaptive colors have a minimum of 7:1 against #fff or #000
	upboundRootStyle         = lipgloss.NewStyle().Foreground(upboundBrandColor)
	pathInactiveSegmentStyle = lipgloss.NewStyle().Foreground(neutralColor)
	pathSegmentStyle         = lipgloss.NewStyle()
)

// NavigationState is a model state that provides a list of items for a navigation node.
type NavigationState interface {
	Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error)
	Breadcrumbs() string
}

// Accepting is a model state that provides a method to accept a navigation node.
type Accepting interface {
	NavigationState
	Accept(upCtx *upbound.Context, writer kubeContextWriter) (string, error)
}

// Back is a model state that provides a method to go back to the parent navigation node.
type Back interface {
	NavigationState
	Back(m model) (model, error)
	CanBack() bool
}

type AcceptingFunc func(ctx context.Context, upCtx *upbound.Context) error

func (f AcceptingFunc) Accept(ctx context.Context, upCtx *upbound.Context) error {
	return f(ctx, upCtx)
}

// breadcrumbStyle defines the styles to be used in the breadcrumbs of a list
type breadcrumbStyle struct {
	// previousLevel is the style of the previous levels in the path (higher
	// order items). For example, when listing control planes then the
	// breadcrumb labels for groups, spaces, orgs and root will be rendered with
	// this style.
	previousLevel lipgloss.Style

	// currentLevel is the style of the current level in the path. For example,
	// when listing control planes then the breadcrumb label for control planes
	// will be rendered with this style.
	currentLevel lipgloss.Style
}

var defaultBreadcrumbStyle = breadcrumbStyle{
	previousLevel: pathInactiveSegmentStyle,
	currentLevel:  pathSegmentStyle,
}

type Root struct{}

func (r *Root) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) { //nolint:gocyclo
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return nil, err
	}

	client := organizations.NewClient(cfg)

	orgs, err := client.List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error listing organizations")
	}

	items := make([]list.Item, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, item{text: org.DisplayName, kind: "organization", matchingTerms: []string{org.Name}, onEnter: func(m model) (model, error) {
			m.state = &Organization{Name: org.Name}
			return m, nil
		}})
	}
	sort.Sort(sortedItems(items))
	return items, nil
}

func (r *Root) breadcrumbs() string {
	return upboundRootStyle.Render("Upbound")
}

func (r *Root) Breadcrumbs() string {
	return r.breadcrumbs()
}

var _ Back = &Organization{}

type Organization struct {
	Name string
}

func (o *Organization) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) { //nolint:gocyclo
	cloudCfg, err := upCtx.BuildControllerClientConfig()
	if err != nil {
		return nil, err
	}

	cloudClient, err := client.New(cloudCfg, client.Options{})
	if err != nil {
		return nil, err
	}

	var l upboundv1alpha1.SpaceList
	err = cloudClient.List(ctx, &l, &client.ListOptions{Namespace: o.Name})
	if err != nil {
		return nil, err
	}

	authInfo, err := getOrgScopedAuthInfo(upCtx, o.Name)
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, 0)
	items = append(items, item{text: "..", kind: "organizations", onEnter: o.Back, back: true})
	for _, space := range l.Items {
		if mode, ok := space.ObjectMeta.Labels[upboundv1alpha1.SpaceModeLabelKey]; ok {
			// todo(redbackthomson): Add support for connected spaces
			if mode == string(upboundv1alpha1.ModeLegacy) || mode == string(upboundv1alpha1.ModeConnected) {
				continue
			}
		}

		items = append(items, item{text: space.GetObjectMeta().GetName(), kind: "space", onEnter: func(m model) (model, error) {
			m.state = &Space{
				Org:     *o,
				Name:    space.GetObjectMeta().GetName(),
				Ingress: space.Status.FQDN,
				// todo(redbackthomson): Replace with public CA data once available
				CA:       make([]byte, 0),
				AuthInfo: authInfo,
			}
			return m, nil
		}})
	}
	sort.Sort(sortedItems(items))
	return items, nil
}

func (o *Organization) Back(m model) (model, error) {
	m.state = &Root{}
	return m, nil
}

func (o *Organization) CanBack() bool {
	return true
}

func (o *Organization) breadcrumbs(styles breadcrumbStyle) string {
	return upboundRootStyle.Render("Upbound ") + styles.previousLevel.Render(fmt.Sprintf("%s/", o.Name))
}

func (o *Organization) Breadcrumbs() string {
	return o.breadcrumbs(defaultBreadcrumbStyle)
}

type sortedItems []list.Item

func (s sortedItems) Len() int           { return len(s) }
func (s sortedItems) Less(i, j int) bool { return s[i].(item).text < s[j].(item).text }
func (s sortedItems) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

var _ Back = &Space{}

// Space provides the navigation node for a space.
type Space struct {
	Org  Organization
	Name string

	Ingress  string
	CA       []byte
	AuthInfo *clientcmdapi.AuthInfo

	// HubContext is an optional field that stores which context in the
	// kubeconfig points at the hub
	HubContext string
}

func (s *Space) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) {
	cl, err := s.GetClient(upCtx)
	if err != nil {
		return nil, err
	}

	nss := &corev1.NamespaceList{}
	if err := cl.List(ctx, nss, client.MatchingLabels(map[string]string{spacesv1beta1.ControlPlaneGroupLabelKey: "true"})); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(nss.Items)+3)
	if s.CanBack() {
		items = append(items, item{text: "..", kind: "spaces", onEnter: s.Back, back: true})
	}
	for _, ns := range nss.Items {
		items = append(items, item{text: ns.Name, kind: "group", onEnter: func(m model) (model, error) {
			m.state = &Group{Space: *s, Name: ns.Name}
			return m, nil
		}})
	}

	if len(nss.Items) == 0 {
		items = append(items, item{text: "No groups found", notSelectable: true})
	}

	items = append(items, item{text: fmt.Sprintf("Switch context to %q", s.Name), onEnter: func(m model) (model, error) {
		msg, err := s.Accept(m.upCtx, m.contextWriter)
		if err != nil {
			return m, err
		}
		return m.WithTermination(msg, nil), nil
	}})

	return items, nil
}

func (s *Space) Back(m model) (model, error) {
	m.state = &s.Org
	return m, nil
}

func (s *Space) IsCloud() bool {
	return s.Org.Name != ""
}

func (s *Space) CanBack() bool {
	return s.IsCloud()
}

func (s *Space) breadcrumbs(styles breadcrumbStyle) string {
	return s.Org.breadcrumbs(breadcrumbStyle{
		currentLevel:  styles.previousLevel,
		previousLevel: styles.previousLevel,
	}) + styles.previousLevel.Render(fmt.Sprintf("%s/", s.Name))
}

func (s *Space) Breadcrumbs() string {
	return s.breadcrumbs(defaultBreadcrumbStyle)
}

// GetClient returns a kube client pointed at the current space
func (s *Space) GetClient(upCtx *upbound.Context) (client.Client, error) {
	conf, err := s.buildClient(upCtx, types.NamespacedName{})
	if err != nil {
		return nil, err
	}

	rest, err := conf.ClientConfig()
	if err != nil {
		return nil, err
	}
	rest.UserAgent = version.UserAgent()

	return client.New(rest, client.Options{})
}

// buildSpacesClient creates a new kubeconfig hardcoded to match the provided
// spaces access configuration and pointed directly at the resource. If the
// resource only specifies a namespace, then the client will point at the space
// hub and the context will be set at the namespace. If the resource specifies
// both a namespace and a name, then the client will point directly at the
// control plane ingress and set the namespace to "default".
// TODO(redbackthomson): Refactor into smaller methods (one for space-level, one
// for ctp-level)
func (s *Space) buildClient(upCtx *upbound.Context, resource types.NamespacedName) (clientcmd.ClientConfig, error) { // nolint:gocyclo
	// reference name for all context, cluster and authinfo for in-memory
	// kubeconfig
	ref := "upbound"

	prev, err := upCtx.Kubecfg.RawConfig()
	if err != nil {
		return nil, err
	}

	config := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		CurrentContext: ref,
		Clusters:       make(map[string]*clientcmdapi.Cluster),
		Contexts:       make(map[string]*clientcmdapi.Context),
		AuthInfos:      make(map[string]*clientcmdapi.AuthInfo),
	}

	refContext := &clientcmdapi.Context{
		Extensions: make(map[string]runtime.Object),
	}

	if resource.Name == "" {
		// point at the space hub
		refContext.Namespace = resource.Namespace

		hubContext, ok := prev.Contexts[s.HubContext]
		if s.HubContext != "" && ok {
			// import the cluster and authinfo from the hub context
			refContext.Cluster = hubContext.Cluster
			config.Clusters[hubContext.Cluster] = ptr.To(*prev.Clusters[hubContext.Cluster])
			refContext.AuthInfo = hubContext.AuthInfo
			config.AuthInfos[hubContext.AuthInfo] = ptr.To(*prev.AuthInfos[hubContext.AuthInfo])
		} else {
			// fall back to ingress if hub context not available
			config.Clusters[ref] = &clientcmdapi.Cluster{
				Server: profile.ToSpacesK8sURL(s.Ingress, resource),
			}
			if len(s.CA) == 0 {
				config.Clusters[ref].InsecureSkipTLSVerify = true
			} else {
				config.Clusters[ref].CertificateAuthorityData = s.CA
			}
			refContext.Cluster = ref

			if s.AuthInfo != nil {
				config.AuthInfos[ref] = s.AuthInfo
				refContext.AuthInfo = ref
			}
		}
	} else {
		// since we are pointing at an individual control plane, point at the
		// "default" namespace inside it
		refContext.Namespace = "default"

		config.Clusters[ref] = &clientcmdapi.Cluster{
			Server: profile.ToSpacesK8sURL(s.Ingress, resource),
		}
		refContext.Cluster = ref

		if len(s.CA) == 0 {
			config.Clusters[ref].InsecureSkipTLSVerify = true
		} else {
			config.Clusters[ref].CertificateAuthorityData = s.CA
		}

		if s.AuthInfo != nil {
			config.AuthInfos[ref] = s.AuthInfo
			refContext.AuthInfo = ref
		} else if s.HubContext != "" {
			hubContext, ok := prev.Contexts[s.HubContext]
			if ok {
				// import the authinfo from the hub context
				refContext.AuthInfo = hubContext.AuthInfo
				config.AuthInfos[hubContext.AuthInfo] = ptr.To(*prev.AuthInfos[hubContext.AuthInfo])
			}
		}
	}

	if s.IsCloud() {
		refContext.Extensions[upbound.ContextExtensionKeySpace] = upbound.NewCloudV1Alpha1SpaceExtension(s.Org.Name)
	} else {
		refContext.Extensions[upbound.ContextExtensionKeySpace] = upbound.NewDisconnectedV1Alpha1SpaceExtension(s.HubContext)
	}

	config.Contexts[ref] = refContext
	return clientcmd.NewDefaultClientConfig(config, &clientcmd.ConfigOverrides{}), nil
}

// Group provides the navigation node for a concrete group aka namespace.
type Group struct {
	Space Space
	Name  string
}

var _ Accepting = &Group{}
var _ Back = &Group{}

func (g *Group) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) {
	cl, err := g.Space.GetClient(upCtx)
	if err != nil {
		return nil, err
	}

	ctps := &spacesv1beta1.ControlPlaneList{}
	if err := cl.List(ctx, ctps, client.InNamespace(g.Name)); err != nil {
		return nil, err
	}

	items := make([]list.Item, 0, len(ctps.Items)+3)
	items = append(items, item{text: "..", kind: "groups", onEnter: g.Back, back: true})

	for _, ctp := range ctps.Items {
		items = append(items, item{text: ctp.Name, kind: "controlplane", onEnter: func(m model) (model, error) {
			m.state = &ControlPlane{Group: *g, Name: ctp.Name}
			return m, nil
		}})
	}

	if len(ctps.Items) == 0 {
		items = append(items, item{text: fmt.Sprintf("No control planes found in the %q group", g.Name), notSelectable: true})
	}

	items = append(items, item{text: fmt.Sprintf("Switch context to %q", fmt.Sprintf("%s/%s", g.Space.Name, g.Name)), onEnter: func(m model) (model, error) {
		msg, err := g.Accept(m.upCtx, m.contextWriter)
		if err != nil {
			return m, err
		}
		return m.WithTermination(msg, nil), nil
	}})

	return items, nil
}

func (g *Group) breadcrumbs(styles breadcrumbStyle) string {
	return g.Space.breadcrumbs(breadcrumbStyle{
		currentLevel:  styles.previousLevel,
		previousLevel: styles.previousLevel,
	}) + styles.previousLevel.Render(fmt.Sprintf("%s/", g.Name))
}

func (g *Group) Breadcrumbs() string {
	return g.breadcrumbs(defaultBreadcrumbStyle)
}

func (g *Group) Back(m model) (model, error) {
	m.state = &g.Space
	return m, nil
}

func (g *Group) CanBack() bool {
	return true
}

// ControlPlane provides the navigation node for a concrete controlplane.
type ControlPlane struct {
	Group Group
	Name  string
}

var _ Accepting = &ControlPlane{}
var _ Back = &ControlPlane{}

func (ctp *ControlPlane) Items(ctx context.Context, upCtx *upbound.Context) ([]list.Item, error) {
	return []list.Item{
		item{text: "..", kind: "controlplanes", onEnter: ctp.Back, back: true},
		item{text: fmt.Sprintf("Connect to %q and quit", ctp.NamespacedName().Name), onEnter: KeyFunc(func(m model) (model, error) {
			msg, err := ctp.Accept(m.upCtx, m.contextWriter)
			if err != nil {
				return m, err
			}
			return m.WithTermination(msg, nil), nil
		})},
	}, nil
}

func (ctp *ControlPlane) breadcrumbs(styles breadcrumbStyle) string {
	// use current level to highlight the entire breadcrumb chain
	return ctp.Group.breadcrumbs(breadcrumbStyle{
		currentLevel:  styles.currentLevel,
		previousLevel: styles.currentLevel,
	}) + pathSegmentStyle.Render(ctp.Name)
}

func (ctp *ControlPlane) Breadcrumbs() string {
	return ctp.breadcrumbs(defaultBreadcrumbStyle)
}

func (ctp *ControlPlane) Back(m model) (model, error) {
	m.state = &ctp.Group
	return m, nil
}

func (ctp *ControlPlane) CanBack() bool {
	return true
}

func (ctp *ControlPlane) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: ctp.Name, Namespace: ctp.Group.Name}
}

func getOrgScopedAuthInfo(upCtx *upbound.Context, orgName string) (*clientcmdapi.AuthInfo, error) {
	// find the current executable path
	cmd, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// if the current executable was the same `up` that is found in PATH
	path, err := exec.LookPath("up")
	if err == nil && path == cmd {
		cmd = "up"
	}

	return &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1",
			Command:    cmd,
			Args:       []string{"organization", "token"},
			Env: []clientcmdapi.ExecEnvVar{
				{
					Name:  "ORGANIZATION",
					Value: orgName,
				},
				{
					Name:  "UP_PROFILE",
					Value: upCtx.ProfileName,
				},
			},
			InteractiveMode: clientcmdapi.IfAvailableExecInteractiveMode,
		},
	}, nil
}
