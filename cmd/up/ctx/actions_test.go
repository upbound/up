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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/upbound"
)

func TestGroupAccept(t *testing.T) {
	tests := map[string]struct {
		conf      *clientcmdapi.Config
		group     string
		preferred string
		wantConf  *clientcmdapi.Config
		wantLast  string
		wantErr   string
	}{
		"ProfileToProfile": {
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
				CurrentContext: "profile",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			wantLast: "profile",
			wantErr:  "<nil>",
		},
		"ProfileGroupToProfileGroup": {
			conf: &clientcmdapi.Config{
				CurrentContext: "group",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"group":            {Namespace: "other", Cluster: "profile", AuthInfo: "profile"},
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
					"upbound": {Namespace: "other", Cluster: "profile", AuthInfo: "profile"},
					"group":   {Namespace: "other", Cluster: "profile", AuthInfo: "profile"},
					"profile": {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			wantLast: "group",
			wantErr:  "<nil>",
		},
		"ProfileToProfileGroup": {
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
			group:     "other",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Namespace: "other", Cluster: "profile", AuthInfo: "profile"},
					"profile": {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			wantLast: "profile",
			wantErr:  "<nil>",
		},
		"UpboundToProfile": {
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
			group:     "group",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "profile",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound",
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
					"upbound":          {Namespace: "other", Cluster: "profile", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
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
					"upbound":          {Namespace: "other", Cluster: "profile", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var last string
			var conf *clientcmdapi.Config
			writer := &fileWriter{
				upCtx:            &upbound.Context{Kubecfg: clientcmd.NewDefaultClientConfig(*tt.conf, nil)},
				kubeContext:      tt.preferred,
				writeLastContext: func(c string) error { last = c; return nil },
				verify:           func(c *clientcmdapi.Config) error { return nil },
				modifyConfig: func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
					conf = &newConfig
					return nil
				},
			}

			g := &Group{
				Space: Space{
					Org:      Organization{Name: "org"},
					Name:     "space",
					Ingress:  "https://ingress",
					CA:       []byte{1, 2, 3},
					AuthInfo: nil,
				},
				Name: tt.group,
			}
			_, err := g.Accept(writer)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("g.accept(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Errorf("g.accept(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Errorf("g.accept(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}

func TestControlPlaneAccept(t *testing.T) {
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
					"upbound": {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"profile": {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
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
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
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
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "default", Cluster: "upbound-previous", AuthInfo: "profile"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"upbound-previous": {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"profile":          {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
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
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			ctp:       types.NamespacedName{Namespace: "group", Name: "ctp2"},
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "default", Cluster: "upbound-previous", AuthInfo: "profile"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound":          {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp2/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"upbound-previous": {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}},
					"profile":          {Server: "profile"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
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
					"upbound":          {Namespace: "default", Cluster: "upbound", AuthInfo: "profile"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound", AuthInfo: "upbound-previous"},
					"profile":          {Namespace: "group", Cluster: "profile", AuthInfo: "profile"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "https://https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/group/controlplanes/ctp/k8s", CertificateAuthorityData: []byte{1, 2, 3}}, "upbound-previous": {Server: "server1"}, "profile": {Server: "profile"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "profile": {Token: "profile"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var last string
			var conf *clientcmdapi.Config
			writer := &fileWriter{
				upCtx:            &upbound.Context{Kubecfg: clientcmd.NewDefaultClientConfig(*tt.conf, nil)},
				kubeContext:      tt.preferred,
				writeLastContext: func(c string) error { last = c; return nil },
				verify:           func(c *clientcmdapi.Config) error { return nil },
				modifyConfig: func(configAccess clientcmd.ConfigAccess, newConfig clientcmdapi.Config, relativizePaths bool) error {
					conf = &newConfig
					return nil
				},
			}

			ctp := &ControlPlane{
				Group: Group{
					Space: Space{
						Org:      Organization{Name: "org"},
						Name:     "space",
						Ingress:  "https://ingress",
						CA:       []byte{1, 2, 3},
						AuthInfo: tt.conf.AuthInfos[tt.conf.Contexts[tt.conf.CurrentContext].AuthInfo],
					},
					Name: tt.ctp.Namespace,
				},
				Name: tt.ctp.Name,
			}
			_, err := ctp.Accept(writer)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("g.accept(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Errorf("g.accept(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Errorf("g.accept(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}
