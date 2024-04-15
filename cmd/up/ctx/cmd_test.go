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
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestSwapContext(t *testing.T) {
	tests := map[string]struct {
		conf      *clientcmdapi.Config
		last      string
		preferred string
		wantConf  *clientcmdapi.Config
		wantLast  string
		wantErr   string
	}{
		"UpboundAndUpboundPrevious": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
					"mixed1":           {Namespace: "mixed1", Cluster: "upbound", AuthInfo: "upbound"},
					"mixed2":           {Namespace: "mixed2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "upbound-previous",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"upbound":          {Namespace: "namespace2", Cluster: "upbound", AuthInfo: "upbound"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
					"mixed1":           {Namespace: "mixed1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"mixed2":           {Namespace: "mixed2", Cluster: "upbound", AuthInfo: "upbound"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server2"}, "upbound-previous": {Server: "server1"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token2"}, "upbound-previous": {Token: "token1"}, "other": {Token: "other"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"OtherAndUpboundPrevious": {
			conf: &clientcmdapi.Config{
				CurrentContext: "other",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
					"mixed1":           {Namespace: "mixed1", Cluster: "upbound", AuthInfo: "upbound"},
					"mixed2":           {Namespace: "mixed2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "upbound-previous",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Namespace: "namespace2", Cluster: "upbound", AuthInfo: "upbound"},
					"other":   {Namespace: "other", Cluster: "other", AuthInfo: "other"},
					"mixed1":  {Namespace: "mixed1", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"mixed2":  {Namespace: "mixed2", Cluster: "upbound", AuthInfo: "upbound"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server2"}, "upbound-previous": {Server: "server1"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token2"}, "upbound-previous": {Token: "token1"}, "other": {Token: "other"}},
			},
			wantLast: "other",
			wantErr:  "<nil>",
		},
		"OtherAndUpbound": {
			conf: &clientcmdapi.Config{
				CurrentContext: "other",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "upbound",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			wantLast: "other",
			wantErr:  "<nil>",
		},
		"UpboundAndOther": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "other",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "other",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			wantLast: "upbound",
			wantErr:  "<nil>",
		},
		"UpboundPreviousAndUpbound": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound-previous",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "upbound",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"UpboundPreviousAndOther": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound-previous",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "other",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "other",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			wantLast: "upbound-previous",
			wantErr:  "<nil>",
		},
		"CurrentNotFound": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound-previous": {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "upbound-previous",
			preferred: "upbound",
			wantErr:   `no "upbound" context found`,
		},
		"LastNotFound": {
			conf: &clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound": {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"other":   {Namespace: "other", Cluster: "other", AuthInfo: "other"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}, "other": {Server: "other"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}, "other": {Token: "other"}},
			},
			last:      "upbound-previous",
			preferred: "upbound",
			wantErr:   `no "upbound-previous" context found`,
		},
		"CustomPreferredContext": {
			conf: &clientcmdapi.Config{
				CurrentContext: "custom",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"custom":           {Namespace: "namespace1", Cluster: "custom", AuthInfo: "custom"},
					"custom-previous":  {Namespace: "namespace2", Cluster: "custom-previous", AuthInfo: "custom-previous"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
					"mixed1":           {Namespace: "mixed1", Cluster: "custom", AuthInfo: "custom"},
					"mixed2":           {Namespace: "mixed2", Cluster: "custom-previous", AuthInfo: "custom-previous"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"custom": {Server: "server1"}, "custom-previous": {Server: "server2"}, "other": {Server: "other"}, "upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"custom": {Token: "token1"}, "custom-previous": {Token: "token2"}, "other": {Token: "other"}, "upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}},
			},
			last:      "custom-previous",
			preferred: "custom",
			wantConf: &clientcmdapi.Config{
				CurrentContext: "custom",
				Contexts: map[string]*clientcmdapi.Context{
					"upbound":          {Namespace: "namespace1", Cluster: "upbound", AuthInfo: "upbound"},
					"upbound-previous": {Namespace: "namespace2", Cluster: "upbound-previous", AuthInfo: "upbound-previous"},
					"custom-previous":  {Namespace: "namespace1", Cluster: "custom-previous", AuthInfo: "custom-previous"},
					"custom":           {Namespace: "namespace2", Cluster: "custom", AuthInfo: "custom"},
					"other":            {Namespace: "other", Cluster: "other", AuthInfo: "other"},
					"mixed1":           {Namespace: "mixed1", Cluster: "custom-previous", AuthInfo: "custom-previous"},
					"mixed2":           {Namespace: "mixed2", Cluster: "custom", AuthInfo: "custom"},
				},
				Clusters:  map[string]*clientcmdapi.Cluster{"custom": {Server: "server2"}, "custom-previous": {Server: "server1"}, "other": {Server: "other"}, "upbound": {Server: "server1"}, "upbound-previous": {Server: "server2"}},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"custom": {Token: "token2"}, "custom-previous": {Token: "token1"}, "other": {Token: "other"}, "upbound": {Token: "token1"}, "upbound-previous": {Token: "token2"}},
			},
			wantLast: "custom-previous",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			conf, last, err := swapContext(tt.conf, tt.last, tt.preferred)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("swapContext(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantConf, conf); diff != "" {
				t.Fatalf("swapContext(...): -want conf, +got conf:\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLast, last); diff != "" {
				t.Fatalf("swapContext(...): -want last, +got last:\n%s", diff)
			}
		})
	}
}
