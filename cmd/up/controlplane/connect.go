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
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/upbound/up/cmd/up/controlplane/kubeconfig"
	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

const (
	upboundPrefix = "upbound_"
)

// connectCmd connects to a control plane by updating the current kubeconfig with
// the control plane's kubeconfig. The disconnect command can be used to restore
// the old context.
type connectCmd struct {
	kubeconfig.ConnectionSecretCmd `cmd:""`
}

func (c *connectCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	return c.ConnectionSecretCmd.AfterApply(kongCtx, upCtx)
}

// Run executes the get command.
func (c *connectCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, getter kubeconfig.ConnectionSecretGetter) error {
	if upCtx.Account == "" {
		return errors.New("error: account is missing from profile")
	}

	// Load kubeconfig from filesystem.
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return err
	}

	// disconnect first if connected
	oldContext := kubeConfig.CurrentContext
	if strings.HasPrefix(oldContext, upboundPrefix) {
		kubeConfig, err = switchToOrigContext(kubeConfig)
		if err != nil {
			return fmt.Errorf("context %q seems to be a control plane context, but disconnect failed: %w", oldContext, err)
		}
		oldContext = kubeConfig.CurrentContext
		if err := clientcmd.ModifyConfig(clientcmd.NewDefaultClientConfigLoadingRules(), kubeConfig, false); err != nil {
			return err
		}
	}

	nname := types.NamespacedName{Namespace: c.Group, Name: c.Name}
	ctpConfig, err := getter.GetKubeConfig(ctx, nname)
	if controlplane.IsNotFound(err) {
		p.Printfln("Control plane %s not found", nname)
		return nil
	}
	if err != nil {
		return err
	}

	expectedContextName := kubeconfig.ExpectedConnectionSecretContext(upCtx.Account, c.Name)
	newKey := controlplaneContextName(upCtx.Account, nname, oldContext)
	ctpConfig, err = kubeconfig.ExtractControlPlaneContext(ctpConfig, expectedContextName, newKey)
	if err != nil {
		return err
	}

	// NOTE(tnthornton) we don't current support supplying files outside of the default system kubeconfig.
	if err := kube.MergeIntoKubeConfig(ctpConfig, "", true, kube.VerifyKubeConfig(upCtx.WrapTransport)); err != nil {
		return err
	}

	p.Printfln("Connected to control plane %s in context %q.\n\nHint: use \"up ctp disconnect\" to restore the previous context.", nname, ctpConfig.CurrentContext)

	return nil
}

func controlplaneContextName(account string, name types.NamespacedName, origCtx string) string {
	if name.Namespace == "" {
		name.Namespace = "default" // passed by value. We can mutate it.
	}
	return fmt.Sprintf("%s%s_%s/%s_%s", upboundPrefix, account, name.Namespace, name.Name, origCtx)
}
