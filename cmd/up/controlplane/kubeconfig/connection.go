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

package kubeconfig

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up-sdk-go/service/configurations"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/controlplane/cloud"
	"github.com/upbound/up/internal/controlplane/space"
	"github.com/upbound/up/internal/upbound"
)

const (
	errFmtConfigBroken = "config is broken, missing %s: %q"
)

// ConnectionSecretCmd is the base for command getting connection secret for a control plane.
type ConnectionSecretCmd struct {
	Name  string `arg:"" required:"" help:"Name of control plane." predictor:"ctps"`
	Token string `help:"API token used to authenticate. Required for Upbound Cloud; ignored otherwise."`
	Group string `short:"g" help:"The control plane group that the control plane is contained in. By default, this is the group specified in the current profile."`

	stdin io.Reader
}

// AfterApply sets default values in command after assignment and validation.
func (c *ConnectionSecretCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	c.stdin = os.Stdin

	var getter ConnectionSecretGetter
	if upCtx.Profile.IsSpace() {
		kubeconfig, ns, err := upCtx.Profile.GetSpaceKubeConfig()
		if err != nil {
			return err
		}
		if c.Group == "" {
			c.Group = ns
		}

		client, err := dynamic.NewForConfig(kubeconfig)
		if err != nil {
			return err
		}
		getter = space.New(client)
	} else {
		if c.Group != "" {
			return fmt.Errorf("group flag is not supported for control plane profile %q", upCtx.ProfileName)
		}

		if c.Token == "" {
			return fmt.Errorf("--token must be specified")
		}

		if c.Token == "-" {
			b, err := io.ReadAll(c.stdin)
			if err != nil {
				return err
			}
			c.Token = strings.TrimSpace(string(b))
		}

		cfg, err := upCtx.BuildSDKConfig()
		if err != nil {
			return err
		}
		ctpclient := cp.NewClient(cfg)
		cfgclient := configurations.NewClient(cfg)

		// The cloud client needs the proxy endpoint and a PAT token for
		// setting up communication with Upbound Cloud.
		getter = cloud.New(
			ctpclient,
			cfgclient,
			upCtx.Account,
			cloud.WithToken(c.Token),
			cloud.WithProxyEndpoint(upCtx.ProxyEndpoint),
		)
	}

	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	kongCtx.BindTo(getter, (*ConnectionSecretGetter)(nil))

	return nil
}

// ExtractControlPlaneContext prunes the given kubeconfig by extracting the one and only
// or the preferred context if there are multiple. It renames context, cluster
// and authInfo to the given key.
func ExtractControlPlaneContext(cfg *api.Config, preferredContextName, newKey string) (*api.Config, error) {
	ctx, ok := cfg.Contexts[preferredContextName]
	if !ok {
		if len(cfg.Contexts) != 1 {
			return nil, fmt.Errorf(errFmtConfigBroken, "context", preferredContextName)
		}

		// fall back if there is only one
		for k := range cfg.Contexts {
			ctx = cfg.Contexts[k]
		}
	}

	cluster, ok := cfg.Clusters[ctx.Cluster]
	if !ok {
		return nil, fmt.Errorf(errFmtConfigBroken, "cluster", ctx.Cluster)
	}
	auth, ok := cfg.AuthInfos[ctx.AuthInfo]
	if !ok {
		return nil, fmt.Errorf(errFmtConfigBroken, "user", ctx.AuthInfo)
	}

	// create new kubeconfig with new keys
	ctx = ctx.DeepCopy()
	ctx.Cluster = newKey
	ctx.AuthInfo = newKey
	return &api.Config{
		Clusters:       map[string]*api.Cluster{newKey: cluster},
		AuthInfos:      map[string]*api.AuthInfo{newKey: auth},
		Contexts:       map[string]*api.Context{newKey: ctx},
		CurrentContext: newKey,
	}, nil
}

func ExpectedConnectionSecretContext(account, ctpName string) string {
	return fmt.Sprintf("%s-%s", account, ctpName)
}
