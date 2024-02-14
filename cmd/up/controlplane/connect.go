// Copyright 2023 Upbound Inc
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

package controlplane

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up-sdk-go/service/configurations"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/controlplane/cloud"
	"github.com/upbound/up/internal/controlplane/space"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	upboundPrefix = "upbound_"

	errFmtConfigBroken = "config is broken, missing %s: %q"
)

type ctpConnector interface {
	GetKubeConfig(ctx context.Context, ctp types.NamespacedName) (*api.Config, error)
}

// AfterApply sets default values in command after assignment and validation.
func (c *connectCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	c.stdin = os.Stdin

	if upCtx.Profile.IsSpace() {
		kubeconfig, ns, err := upCtx.Profile.GetKubeClientConfig()
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
		c.client = space.New(client)
	} else {
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
		c.client = cloud.New(
			ctpclient,
			cfgclient,
			upCtx.Account,
			cloud.WithToken(c.Token),
			cloud.WithProxyEndpoint(upCtx.ProxyEndpoint),
		)
	}

	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))
	return nil
}

// getCmd gets a single control plane in an account on Upbound.
type connectCmd struct {
	Name  string `arg:"" required:"" help:"Name of control plane." predictor:"ctps"`
	Token string `help:"API token used to authenticate. Required for Upbound Cloud; ignored otherwise."`

	Group string `short:"g" help:"The control plane group that the control plane is contained in. By default, this is the group specified in the current profile."`

	stdin  io.Reader
	client ctpConnector
}

// Run executes the get command.
func (c *connectCmd) Run(ctx context.Context, printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context) error {
	if upCtx.Account == "" {
		return errors.New("error: account is missing from profile")
	}

	// Load kubeconfig from filesystem.
	kcloader, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return err
	}
	// Check if the fs kubeconfig is already pointing to a control plane and
	// return early if so.
	origContext := kcloader.CurrentContext
	if strings.HasPrefix(origContext, upboundPrefix) {
		return fmt.Errorf("already connected to a control plane. Disconnect first")
	}

	cfg, err := c.client.GetKubeConfig(ctx, types.NamespacedName{Namespace: c.Group, Name: c.Name})
	if controlplane.IsNotFound(err) {
		p.Printfln("Control plane %s not found", c.Name)
		return nil
	}
	if err != nil {
		return err
	}

	modifiedCfg, err := updateKubeConfig(*cfg, upCtx.Account, c.Name, origContext)
	if err != nil {
		return err
	}
	// NOTE(tnthornton) we don't current support supplying files outside of
	// the default system kubeconfig.
	if err := kube.ApplyControlPlaneKubeconfig(modifiedCfg, "", upCtx.WrapTransport); err != nil {
		return err
	}

	p.Printfln("Current context set to %s", modifiedCfg.CurrentContext)

	return nil
}

// updateKubeConfig updates the given kubeconfig with new cluster, user, and
// context keys based on the account, control plane name, and original context
// that are provided.
func updateKubeConfig(cfg api.Config, account, ctpName, origContext string) (api.Config, error) {
	// Grab the context from control plane kubeconfig.
	sourceKey := defaultContextName(account, ctpName)
	ctpKey := controlplaneContextName(account, ctpName, origContext)

	// Update the context, cluster, and user names in the control plane
	// config.
	cluster, ok := cfg.Clusters[sourceKey]
	if !ok {
		return api.Config{}, fmt.Errorf(errFmtConfigBroken, "cluster", sourceKey)
	}
	cfg.Clusters[ctpKey] = cluster
	delete(cfg.Clusters, sourceKey)

	users, ok := cfg.AuthInfos[sourceKey]
	if !ok {
		return api.Config{}, fmt.Errorf(errFmtConfigBroken, "user", sourceKey)
	}
	cfg.AuthInfos[ctpKey] = users
	delete(cfg.AuthInfos, sourceKey)

	// Rename context, move under upbound- namespace.
	context, ok := cfg.Contexts[sourceKey]
	if !ok {
		return api.Config{}, fmt.Errorf(errFmtConfigBroken, "context", sourceKey)
	}
	context.AuthInfo = ctpKey
	context.Cluster = ctpKey
	cfg.Contexts[ctpKey] = context
	delete(cfg.Contexts, sourceKey)

	// Update current context in the config to the built key. The next step
	// will update the fs kubeconfig based on these details and without this
	// step the update will fail due to the current context being set to the
	// sourceKey value above.
	cfg.CurrentContext = ctpKey
	return cfg, nil
}

func defaultContextName(account, ctpName string) string {
	return fmt.Sprintf("%s-%s", account, ctpName)
}

func controlplaneContextName(account, ctpName, origCtx string) string {
	return fmt.Sprintf("%s%s_%s_%s", upboundPrefix, account, ctpName, origCtx)
}
