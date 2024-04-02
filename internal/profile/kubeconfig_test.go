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

package profile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestFindProfileByURL(t *testing.T) {
	getIngressHostFn := func(ingress string, err error) func(ctx context.Context, cfg *rest.Config) (string, error) {
		return func(ctx context.Context, cfg *rest.Config) (string, error) {
			return ingress, err
		}
	}
	tests := map[string]struct {
		reason         string
		profiles       map[string]Profile
		conf           *clientcmdapi.Config
		getIngressHost func(ctx context.Context, cfg *rest.Config) (string, error)
		wantProfile    *Profile
		wantCtp        types.NamespacedName
		wantErr        string
	}{
		"NoProfile": {
			reason: "no profiles give error",
			conf: &clientcmdapi.Config{
				CurrentContext: "foo",
				Contexts:       map[string]*clientcmdapi.Context{"foo": {Cluster: "foo", AuthInfo: "foo"}},
				Clusters:       map[string]*clientcmdapi.Cluster{"foo": {Server: "https://foo.com"}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"foo": {}},
			},
			wantProfile: nil,
		},
		"UnknownContext": {
			reason: "context not in kubeconfig",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "missing",
				Contexts:       map[string]*clientcmdapi.Context{"foo": {Cluster: "foo", AuthInfo: "foo"}},
				Clusters:       map[string]*clientcmdapi.Cluster{"foo": {Server: "https://foo.com"}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"foo": {}},
			},
			wantErr: "current context \"missing\" not found in kubeconfig",
		},
		"UnknownCluster": {
			reason: "cluster not in kubeconfig",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "foo",
				Contexts:       map[string]*clientcmdapi.Context{"foo": {Cluster: "missing", AuthInfo: "foo"}},
				Clusters:       map[string]*clientcmdapi.Cluster{"bar": {Server: "https://bar.com"}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"foo": {}},
			},
			wantErr: "cluster \"missing\" not found in kubeconfig",
		},
		"OneMatchingProfile": {
			reason: "an exact match on URL, defaulting to default namespace",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "foo",
				Contexts:       map[string]*clientcmdapi.Context{"foo": {Cluster: "foo", AuthInfo: "foo"}},
				Clusters:       map[string]*clientcmdapi.Cluster{"foo": {Server: "https://foo.com"}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"foo": {}},
			},
			getIngressHost: getIngressHostFn("https://foo.com", nil),
			wantProfile:    &Profile{ID: "foo", Type: "space", KubeContext: "foo"},
			wantCtp:        types.NamespacedName{Namespace: "default"},
		},
		"NonMatchingProfile": {
			reason: "profile URL does not match kubeconfig URL",
			profiles: map[string]Profile{
				"bar": {ID: "bar", Type: "space", KubeContext: "bar"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "foo",
				Contexts: map[string]*clientcmdapi.Context{
					"foo": {Cluster: "foo", AuthInfo: "foo"},
					"bar": {Cluster: "bar", AuthInfo: "bar"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {Server: "https://foo.com"},
					"bar": {Server: "https://bar.com"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"foo": {}, "bar": {}},
			},
			getIngressHost: getIngressHostFn("https://bar.com", nil),
			wantProfile:    nil,
		},
		"OtherContextName": {
			reason: "profile context name does not match kubeconfig context name, but URL matches",
			profiles: map[string]Profile{
				"bar": {ID: "bar", Type: "space", KubeContext: "bar"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "foo",
				Contexts: map[string]*clientcmdapi.Context{
					"foo": {Cluster: "foo", AuthInfo: "foo"},
					"bar": {Cluster: "bar", AuthInfo: "bar"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {Server: "https://foo.com"},
					"bar": {Server: "https://foo.com"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"foo": {}, "bar": {}},
			},
			getIngressHost: getIngressHostFn("https://bar.com", nil),
			wantProfile:    &Profile{ID: "bar", Type: "space", KubeContext: "bar"},
			wantCtp:        types.NamespacedName{Namespace: "default"},
		},
		"Group": {
			reason: "full group URL, namespace from kubeconfig",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "foo",
				Contexts:       map[string]*clientcmdapi.Context{"foo": {Cluster: "foo", AuthInfo: "foo", Namespace: "group"}},
				Clusters:       map[string]*clientcmdapi.Cluster{"foo": {Server: "https://foo.com"}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"foo": {}},
			},
			getIngressHost: getIngressHostFn("https://bar.com", nil),
			wantProfile:    &Profile{ID: "foo", Type: "space", KubeContext: "foo"},
			wantCtp:        types.NamespacedName{Namespace: "group"},
		},
		"ControlPlane": {
			reason: "full controlplane URL",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "bar",
				Contexts: map[string]*clientcmdapi.Context{
					"foo": {Cluster: "foo", AuthInfo: "foo"},
					"bar": {Cluster: "bar", AuthInfo: "bar"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {Server: "https://foo.com"},
					"bar": {Server: "https://foo.com/apis/spaces.upbound.io/v1alpha1/namespaces/group/controlplanes/foo/k8s"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"foo": {}, "bar": {}},
			},
			getIngressHost: getIngressHostFn("https://bar.com", nil),
			wantProfile:    &Profile{ID: "foo", Type: "space", KubeContext: "foo"},
			wantCtp:        types.NamespacedName{Namespace: "group", Name: "foo"},
		},
		"ControlPlaneIngress": {
			reason: "full controlplane URL with ingress host, resolved via mxp-config",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "bar",
				Contexts: map[string]*clientcmdapi.Context{
					"foo": {Cluster: "foo", AuthInfo: "foo"},
					"bar": {Cluster: "bar", AuthInfo: "bar"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {Server: "https://foo.com"},
					"bar": {Server: "https://bar.com/apis/spaces.upbound.io/v1alpha1/namespaces/group/controlplanes/foo/k8s"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"foo": {}, "bar": {}},
			},
			getIngressHost: getIngressHostFn("https://bar.com", nil),
			wantProfile:    &Profile{ID: "foo", Type: "space", KubeContext: "foo"},
			wantCtp:        types.NamespacedName{Namespace: "group", Name: "foo"},
		},
		"GetIngressError": {
			reason: "getIngressHost errors, hence no match found",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "bar",
				Contexts: map[string]*clientcmdapi.Context{
					"foo": {Cluster: "foo", AuthInfo: "foo"},
					"bar": {Cluster: "bar", AuthInfo: "bar"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {Server: "https://foo.com"},
					"bar": {Server: "https://bar.com/apis/spaces.upbound.io/v1alpha1/namespaces/group/controlplanes/foo/k8s"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"foo": {}, "bar": {}},
			},
			getIngressHost: getIngressHostFn("", errors.New("get mxp-config error")),
			wantProfile:    nil,
		},
		"ControlPlaneIngressSlash": {
			reason: "full controlplane URL with ingress host, resolved via mxp-config",
			profiles: map[string]Profile{
				"foo": {ID: "foo", Type: "space", KubeContext: "foo"},
			},
			conf: &clientcmdapi.Config{
				CurrentContext: "bar",
				Contexts: map[string]*clientcmdapi.Context{
					"foo": {Cluster: "foo", AuthInfo: "foo"},
					"bar": {Cluster: "bar", AuthInfo: "bar"},
				},
				Clusters: map[string]*clientcmdapi.Cluster{
					"foo": {Server: "https://foo.com/"},
					"bar": {Server: "https://bar.com/apis/spaces.upbound.io/v1alpha1/namespaces/group/controlplanes/foo/k8s/"},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"foo": {}, "bar": {}},
			},
			getIngressHost: getIngressHostFn("https://bar.com/", nil),
			wantProfile:    &Profile{ID: "foo", Type: "space", KubeContext: "foo"},
			wantCtp:        types.NamespacedName{Namespace: "group", Name: "foo"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, p, ctp, err := findProfileByKubeconfig(context.Background(), tt.profiles, tt.conf, tt.getIngressHost)

			if diff := cmp.Diff(tt.wantErr == "", err == nil); diff != "" {
				t.Errorf("findProfileByURL() -want error, +got error:\n%v\n%v", diff, err)
				return
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("findProfileByURL() -contains %q:\n%v", tt.wantErr, err)
			}
			if diff := cmp.Diff(tt.wantProfile, p); diff != "" {
				t.Errorf("findProfileByURL() -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantCtp, ctp); diff != "" {
				t.Errorf("findProfileByURL() -want, +got:\n%s", diff)
			}
		})
	}
}
