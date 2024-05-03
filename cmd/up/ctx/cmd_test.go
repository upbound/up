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

	"github.com/golang-jwt/jwt"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/upbound/up/internal/upbound"
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
		"UpboundAndUpbound": {
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
			last:      "upbound",
			preferred: "upbound",
			wantConf: &clientcmdapi.Config{
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
			wantLast: "upbound",
			wantErr:  "<nil>",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			conf, last, err := activateContext(tt.conf, tt.last, tt.preferred)
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

func TestDeriveState(t *testing.T) {
	hubCA := `
-----BEGIN CERTIFICATE-----
MIIDNzCCAh+gAwIBAgIIMPmY2QCCgcYwDQYJKoZIhvcNAQELBQAwIjEgMB4GA1UE
AwwXMTI3LjAuMC4xLWNhQDE2OTkxOTMzMzgwIBcNMjMxMTA1MTMwODU4WhgPMjEy
MzEwMTIxMzA4NThaMB8xHTAbBgNVBAMMFDEyNy4wLjAuMUAxNjk5MTkzMzM4MIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzPVMYesXhGL3YQlmNeft2oIg
CmfXQaJee34G4OL7G8NIjkU9XJVhqLGtU/gNRY9+vB/k8NZLF+xipJT5GVzFMu+o
tJeMHuFYB+2iMNINPMWhEAOqa9kSGDsUzH2gZVjZZiz/paWf54iAGW0L5urXLqFh
hTsHGvIk8qdln3HxxNN3nwB+6jXjzbGSJ7XLYFiQcsCtjbyzFNxdnMuYeNbOvxK/
GWCWF27NP1/vT+7XudcrXvtDcgqG5Zf4oq45Wheeo1vZaYJUOX29zpMX4cZ7KnKp
bDOSTW9KHeRP8YpPa6tnq0Irpj2FNEha/ouJRYxXN7ACzKmChR3fn24k9n8P5QID
AQABo3IwcDAOBgNVHQ8BAf8EBAMCBaAwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDAYD
VR0TAQH/BAIwADAfBgNVHSMEGDAWgBQJUtOqYZLkhCSCT3ILBfptuUZMaTAaBgNV
HREEEzARgglsb2NhbGhvc3SHBH8AAAEwDQYJKoZIhvcNAQELBQADggEBAJb7OSze
+Zq+fPS1wQ2YKELtLtJ2r49VdgC+UMxw0pggEID1dRM+A9jm3m7mA099OpmQK9AO
TlFKZHtZl+PV6oTA5Wd7gg9YUNenECgcHfMVJvtr5ctH+ynVGrPbxXSrJBWuxBZk
bmTQVoNz1SdOXn1aRjqH6GgDQJh8UZUMjlmusYGoWHt/vFRcJS8fY6M3ANf7OGFd
cuRD2TNaJprYCB9Q7yvybTNYOh2STnTyzRxM2vxmYmGtyOVW5Eu6Ut5VPS/Jgli1
LAOjVgGvSuiuM72Cr2qQgc7Q5ke4M0DG90Qr/DZMSlc4US1Ba++cy3+8n3puxbIg
9X+1x5wP0N2O06Y=
-----END CERTIFICATE-----
`
	ingressCA := `
-----BEGIN CERTIFICATE-----
MIIDCjCCAfKgAwIBAgIIGB6xU7MT5AAwDQYJKoZIhvcNAQELBQAwIjEgMB4GA1UE
AwwXMTI3LjAuMC4xLWNhQDE2OTkxOTMzMzgwIBcNMjMxMTA1MTMwODU4WhgPMjEy
MzEwMTIxMzA4NThaMCIxIDAeBgNVBAMMFzEyNy4wLjAuMS1jYUAxNjk5MTkzMzM4
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0OHCeG2uTE4y6ce98V/3
6M/jnVxcYYNSSciAHAlLEIyrCzQbsGWWcdaAMsXhlJ2ZrXLMV8pRYCqNpVzA981T
YK1ODuJfCaOldppb9HPrw3Q7rTVLxjGBL5T0gnaPsxqglVS3hBAbkPtuOGV0Fl/Z
JMJcYR4WUxe0jyLwD4+tftT2Rso72wGMqhItSF4EqbLd3vf7qWgjFgFNL4Ggqsy4
hDWmOQNg1CGOGa2140JKDhqIBZ23Xefns2yaZ8u/F14jyjmJ/BwTAywRB+0RwtjZ
HAAIocu3XKUoJeQO1dvT91YrzQ+THHA5W6XMonnYZj0majkWG5fqqDEmtky8lHWm
XQIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNV
HQ4EFgQUCVLTqmGS5IQkgk9yCwX6bblGTGkwDQYJKoZIhvcNAQELBQADggEBADtq
EpQ5jEnr4vepbeZ2QCyX/2OxdSKlWzK2YA1cMThooQKbGZ43POa15n4lD6uMViXy
yZTbzP8sWQ3kJpj252pm9KuO8uv3w5zxgL/aVdu6+k/EzpWab2jsR7Fzuj3dDYTM
aU88g5QpmUX3xtP7HqVwl+LzZuZpM8U7il8PWGyraDnniSAYfp9pp5lViPN2IPP9
ORaAbHyljalRFcjEDBwZtSBo3zcaA12uKtaEoFZShU0PDKCFCJ1weyqEI/Jmoays
xPWjLExASVeAdNehjgFcrfoc7ZWtJYeE42his0athGjS/fNK7PnjijpZn6h76hRB
92l9SyA6+IXPGFmjFUU=
-----END CERTIFICATE-----
`

	ingressPublicNotFound := func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
		return "", nil, errors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "ingress-public")
	}
	ingressUnknownKind := func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
		return "", nil, &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "ConfigMap"}}
	}
	orgToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.MapClaims{"organization": "org"}).SignedString([]byte("key"))
	if err != nil {
		t.Fatalf("jwt.NewWithClaims(...): %v", err)
	}

	tests := map[string]struct {
		conf           clientcmdapi.Config
		getIngressHost func(ctx context.Context, cl client.Client) (host string, ca []byte, err error)

		want    NavigationState
		wantErr string
	}{
		"HubWithoutIngressPublic": {
			conf: clientcmdapi.Config{
				CurrentContext: "hub",
				Contexts:       map[string]*clientcmdapi.Context{"hub": {Namespace: "default", Cluster: "hub", AuthInfo: "hub"}},
				Clusters:       map[string]*clientcmdapi.Cluster{"hub": {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: ingressPublicNotFound,
			want:           &Root{},
			wantErr:        "<nil>",
		},
		"HubWithIngressPublic": {
			conf: clientcmdapi.Config{
				CurrentContext: "hub",
				Contexts:       map[string]*clientcmdapi.Context{"hub": {Namespace: "default", Cluster: "hub", AuthInfo: "hub"}},
				Clusters:       map[string]*clientcmdapi.Cluster{"hub": {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)}},
				AuthInfos:      map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
				return "https://ingress", []byte(ingressCA), nil
			},
			want: &Group{
				Space: Space{
					Name:    "hub",
					Ingress: "https://ingress",
					CA:      []byte(ingressCA),
					AuthInfo: &clientcmdapi.AuthInfo{
						Token: "token",
					},
				},
				Name: "default",
			},
			wantErr: "<nil>",
		},
		"IngressWithIngressPublic": {
			conf: clientcmdapi.Config{
				CurrentContext: "ingress",
				Contexts:       map[string]*clientcmdapi.Context{"ingress": {Namespace: "default", Cluster: "ingress", AuthInfo: "hub"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"hub":     {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)},
					"ingress": {Server: "https://ingress", CertificateAuthorityData: []byte(ingressCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
				return "https://ingress", []byte(ingressCA), nil
			},
			want: &Group{
				Space: Space{
					Name:    "ingress",
					Ingress: "https://ingress",
					CA:      []byte(ingressCA),
					AuthInfo: &clientcmdapi.AuthInfo{
						Token: "token",
					},
				},
				Name: "default",
			},
			wantErr: "<nil>",
		},
		"WithoutNamespace": {
			conf: clientcmdapi.Config{
				CurrentContext: "ingress",
				Contexts:       map[string]*clientcmdapi.Context{"ingress": {Namespace: "", Cluster: "ingress", AuthInfo: "hub"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"hub":     {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)},
					"ingress": {Server: "https://ingress", CertificateAuthorityData: []byte(ingressCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
				return "https://ingress", []byte(ingressCA), nil
			},
			want: &Space{
				Ingress: "https://ingress",
				CA:      []byte(ingressCA),
				AuthInfo: &clientcmdapi.AuthInfo{
					Token: "token",
				},
				Name: "ingress",
			},
			wantErr: "<nil>",
		},
		"ControlPlane": {
			conf: clientcmdapi.Config{
				CurrentContext: "ctp1",
				Contexts:       map[string]*clientcmdapi.Context{"ctp1": {Namespace: "default", Cluster: "ctp1", AuthInfo: "hub"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"hub":     {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)},
					"ingress": {Server: "https://ingress", CertificateAuthorityData: []byte(ingressCA)},
					"ctp1":    {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp1/k8s", CertificateAuthorityData: []byte(ingressCA)},
					"ctp2":    {Server: "https://ingress/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp2/k8s", CertificateAuthorityData: []byte(ingressCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
				return "https://ingress", []byte(ingressCA), nil
			},
			want: &ControlPlane{
				Group: Group{
					Space: Space{
						Ingress: "https://ingress",
						CA:      []byte(ingressCA),
						AuthInfo: &clientcmdapi.AuthInfo{
							Token: "token",
						},
						Name: "ctp1",
					},
					Name: "default",
				},
				Name: "ctp1",
			},
			wantErr: "<nil>",
		},
		"CloudSpace": {
			conf: clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts:       map[string]*clientcmdapi.Context{"upbound": {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound": {Server: "https://eu-west-1.ibm-cloud.com", CertificateAuthorityData: []byte(ingressCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: orgToken}},
			},
			getIngressHost: ingressPublicNotFound,
			want: &Group{
				Space: Space{
					Org: Organization{
						Name: "org",
					},
					Name:     "eu-west-1", // TODO: where does this come from?
					Ingress:  "eu-west-1.ibm-cloud.com",
					CA:       []byte(ingressCA),
					AuthInfo: &clientcmdapi.AuthInfo{Token: orgToken},
				},
				Name: "default",
			},
			wantErr: "<nil>",
		},
		"CloudControlPlane": {
			conf: clientcmdapi.Config{
				CurrentContext: "upbound",
				Contexts:       map[string]*clientcmdapi.Context{"upbound": {Namespace: "default", Cluster: "upbound", AuthInfo: "upbound"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"upbound": {Server: "https://eu-west-1.ibm-cloud.com/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp1/k8s", CertificateAuthorityData: []byte(ingressCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"upbound": {Token: orgToken}},
			},
			getIngressHost: ingressUnknownKind,
			want: &ControlPlane{
				Group: Group{
					Space: Space{
						Org: Organization{
							Name: "org",
						},
						Name:     "eu-west-1",
						Ingress:  "eu-west-1.ibm-cloud.com",
						CA:       []byte(ingressCA),
						AuthInfo: &clientcmdapi.AuthInfo{Token: orgToken},
					},
					Name: "default",
				},
				Name: "ctp1",
			},
			wantErr: "<nil>",
		},
		"UnknownCluster": {
			conf: clientcmdapi.Config{
				CurrentContext: "hub",
				Contexts:       map[string]*clientcmdapi.Context{"hub": {Namespace: "default", Cluster: "invalid", AuthInfo: "hub"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"hub": {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
				return "https://ingress", []byte(ingressCA), nil
			},
			want:    nil,
			wantErr: `invalid configuration: cluster "invalid" was not found for context "hub"`,
		},
		"UnknownAuthInfo": {
			conf: clientcmdapi.Config{
				CurrentContext: "hub",
				Contexts:       map[string]*clientcmdapi.Context{"hub": {Namespace: "default", Cluster: "hub", AuthInfo: "invalid"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"hub": {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
				return "https://ingress", []byte(ingressCA), nil
			},
			want:    nil,
			wantErr: `invalid configuration: user "invalid" was not found for context "hub"`,
		},
		"UnknownContext": {
			conf: clientcmdapi.Config{
				CurrentContext: "invalid",
				Contexts:       map[string]*clientcmdapi.Context{"hub": {Namespace: "default", Cluster: "hub", AuthInfo: "hub"}},
				Clusters: map[string]*clientcmdapi.Cluster{
					"hub": {Server: "https://hub", CertificateAuthorityData: []byte(hubCA)},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{"hub": {Token: "token"}},
			},
			getIngressHost: func(ctx context.Context, cl client.Client) (host string, ca []byte, err error) {
				return "https://ingress", []byte(ingressCA), nil
			},
			want:    nil, // or do we want an error?
			wantErr: `invalid configuration: context was not found for specified context: invalid`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			upCtx := &upbound.Context{
				Kubecfg: clientcmd.NewDefaultClientConfig(tt.conf, nil),
			}
			got, err := deriveState(context.Background(), upCtx, &tt.conf, tt.getIngressHost)
			if diff := cmp.Diff(tt.wantErr, fmt.Sprintf("%v", err)); diff != "" {
				t.Fatalf("DeriveState(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("swapContext(...): -want conf, +got conf:\n%s", diff)
			}
		})
	}
}
