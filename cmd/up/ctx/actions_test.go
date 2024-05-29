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
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up/internal/spaces"
	"github.com/upbound/up/internal/upbound"
)

func TestDisconnectedGroupAccept(t *testing.T) {
	spaceExtension := upbound.NewDisconnectedV1Alpha1SpaceExtension("profile")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}

	tests := map[string]struct {
		conf      *clientcmdapi.Config
		group     string
		preferred string
		wantConf  *clientcmdapi.Config
		wantLast  string
		wantErr   string
	}{
		"ProfileToUpbound": {
			conf: &clientcmdapi.Config{
				CurrentContext: "profile",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			group:     "group",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Namespace: "group", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"profile": {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "profile"}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "profile",
			wantErr:  "<nil>",
		},
		"UpboundToDifferentGroup": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			group:     "other",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "other", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "profile"}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"UpboundPreviousToDifferentGroup": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound-previous",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			group:     "other",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "other", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound", AuthInfo: "upbound"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "profile"}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var last string
			var conf *clientcmdapi.Config
			upCtx := &upbound.Context{Kubecfg: clientcmd.NewDefaultClientConfig(*tt.conf, nil)}
			writer := &fileWriter{
				upCtx:            upCtx,
				kubeContext:      tt.preferred,
				writeLastContext: func(c string) error { last = c; return nil },
				verify:           func(c *clientcmdapi.Config) error { return nil },
				modifyConfig: func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
					conf = &newConfig
					return nil
				},
			}
			navCtx := &navContext{
				contextWriter: writer,
				ingressReader: &mockIngressReader{},
			}

			g := &Group{
				Space: Space{
					Name: "space",
					Ingress: spaces.SpaceIngress{
						Host:   "ingress",
						CAData: []byte{1, 2, 3},
					},
					HubContext: "profile",
				},
				Name: tt.group,
			}
			_, err := g.Accept(upCtx, navCtx)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("g.Accept(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Errorf("g.Accept(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Errorf("g.Accept(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}

func TestCloudGroupAccept(t *testing.T) {
	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("org")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}
	spaceAuth := clientcmdapi.AuthInfo{Token: "space"}

	tests := map[string]struct {
		conf      *clientcmdapi.Config
		group     string
		preferred string
		wantConf  *clientcmdapi.Config
		wantLast  string
		wantErr   string
	}{
		"ProfileToUpbound": {
			conf: &clientcmdapi.Config{
				CurrentContext: "profile",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			group:     "group",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Namespace: "group", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"profile": {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": &spaceAuth, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "profile",
			wantErr:  "<nil>",
		},
		"UpboundToDifferentGroup": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			group:     "other",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "other", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": &spaceAuth, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"UpboundPreviousToDifferentGroup": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound-previous",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			group:     "other",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "other", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound", AuthInfo: "upbound"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress", CertificateAuthorityData: []uint8{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": &spaceAuth, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var last string
			var conf *clientcmdapi.Config
			upCtx := &upbound.Context{Kubecfg: clientcmd.NewDefaultClientConfig(*tt.conf, nil)}
			writer := &fileWriter{
				upCtx:            upCtx,
				kubeContext:      tt.preferred,
				writeLastContext: func(c string) error { last = c; return nil },
				verify:           func(c *clientcmdapi.Config) error { return nil },
				modifyConfig: func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
					conf = &newConfig
					return nil
				},
			}
			navCtx := &navContext{
				contextWriter: writer,
				ingressReader: &mockIngressReader{},
			}

			g := &Group{
				Space: Space{
					Org:  Organization{Name: "org"},
					Name: "space",
					Ingress: spaces.SpaceIngress{
						Host:   "ingress",
						CAData: []byte{1, 2, 3},
					},
					AuthInfo: &spaceAuth,
				},
				Name: tt.group,
			}
			_, err := g.Accept(upCtx, navCtx)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("g.Accept(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Errorf("g.Accept(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Errorf("g.Accept(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}

func TestDisconnectedControlPlaneAccept(t *testing.T) {
	spaceExtension := upbound.NewDisconnectedV1Alpha1SpaceExtension("profile")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}

	tests := map[string]struct {
		conf      *clientcmdapi.Config
		ctp       types.NamespacedName
		preferred string
		wantConf  *clientcmdapi.Config
		wantLast  string
		wantErr   string
	}{
		"ProfileToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "profile",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"profile": {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "profile",
			wantErr:  "<nil>",
		},
		"UpboundToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"ControlPlaneToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "default", Cluster: "upbound-previous", AuthInfo: "profile"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"upbound-previous": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"profile":          {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"ControlPlaneToDifferentControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp2"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "default", Cluster: "upbound-previous", AuthInfo: "profile"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp2/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"upbound-previous": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"profile":          {Server: "profile"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"UpboundPreviousToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound-previous",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound", AuthInfo: "upbound"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var last string
			var conf *clientcmdapi.Config
			upCtx := &upbound.Context{Kubecfg: clientcmd.NewDefaultClientConfig(*tt.conf, nil)}
			writer := &fileWriter{
				upCtx:            upCtx,
				kubeContext:      tt.preferred,
				writeLastContext: func(c string) error { last = c; return nil },
				verify:           func(c *clientcmdapi.Config) error { return nil },
				modifyConfig: func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
					conf = &newConfig
					return nil
				},
			}
			navCtx := &navContext{
				contextWriter: writer,
				ingressReader: &mockIngressReader{},
			}

			ctp := &ControlPlane{
				Group: Group{
					Space: Space{
						Name: "space",
						Ingress: spaces.SpaceIngress{
							Host:   "ingress",
							CAData: []byte{1, 2, 3},
						},
						HubContext: "profile",
					},
					Name: tt.ctp.Namespace,
				},
				Name: tt.ctp.Name,
			}
			_, err := ctp.Accept(upCtx, navCtx)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("g.Accept(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Errorf("g.Accept(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Errorf("g.Accept(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}

func TestCloudControlPlaneAccept(t *testing.T) {
	spaceExtension := upbound.NewCloudV1Alpha1SpaceExtension("org")
	extensionMap := map[string]runtime.Object{upbound.ContextExtensionKeySpace: spaceExtension}

	tests := map[string]struct {
		conf      *clientcmdapi.Config
		ctp       types.NamespacedName
		preferred string
		wantConf  *clientcmdapi.Config
		wantLast  string
		wantErr   string
	}{
		"ProfileToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "profile",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"profile": {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "profile",
			wantErr:  "<nil>",
		},
		"UpboundToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"ControlPlaneToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "default", Cluster: "upbound-previous", AuthInfo: "profile"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"upbound-previous": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"profile":          {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"ControlPlaneToDifferentControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp2"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "default", Cluster: "upbound-previous", AuthInfo: "profile"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp2/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"upbound-previous": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"profile":          {Server: "profile"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "profile"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"UpboundPreviousToControlPlane": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound-previous",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound", Extensions: extensionMap},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound", AuthInfo: "upbound"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token2"}, "upbound-previous": {Token: "token1"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var last string
			var conf *clientcmdapi.Config
			upCtx := &upbound.Context{Kubecfg: clientcmd.NewDefaultClientConfig(*tt.conf, nil)}
			writer := &fileWriter{
				upCtx:            upCtx,
				kubeContext:      tt.preferred,
				writeLastContext: func(c string) error { last = c; return nil },
				verify:           func(c *clientcmdapi.Config) error { return nil },
				modifyConfig: func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
					conf = &newConfig
					return nil
				},
			}
			navCtx := &navContext{
				contextWriter: writer,
				ingressReader: &mockIngressReader{},
			}

			ctp := &ControlPlane{
				Group: Group{
					Space: Space{
						Org:  Organization{Name: "org"},
						Name: "space",
						Ingress: spaces.SpaceIngress{
							Host:   "ingress",
							CAData: []byte{1, 2, 3},
						},
						AuthInfo: tt.conf.AuthInfos[tt.conf.Contexts[tt.conf.CurrentContext].AuthInfo],
					},
					Name: tt.ctp.Namespace,
				},
				Name: tt.ctp.Name,
			}
			_, err := ctp.Accept(upCtx, navCtx)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("g.Accept(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Errorf("g.Accept(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Errorf("g.Accept(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}

var _ spaces.IngressReader = &mockIngressReader{}

type mockIngressReader struct{}

func (m *mockIngressReader) Get(ctx context.Context, space v1alpha1.Space) (ingress *spaces.SpaceIngress, err error) {
	return &spaces.SpaceIngress{
		Host:   "ingress",
		CAData: []byte{1, 2, 3},
	}, nil
}
