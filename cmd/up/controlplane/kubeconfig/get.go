// Copyright 2021 Upbound Inc
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
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/controlplane"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// getCmd gets kubeconfig data for an Upbound control plane.
type getCmd struct {
	ConnectionSecretCmd

	File    string `type:"path" short:"f" help:"File to merge control plane kubeconfig into or to create. By default it is merged into the user's default kubeconfig. Use '-' to print it to stdout.'"`
	Context string `short:"c" help:"Context to use in the kubeconfig."`
}

func (c *getCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	return c.ConnectionSecretCmd.AfterApply(kongCtx, upCtx)
}

// Run executes the get command.
func (c *getCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context, getter ConnectionSecretGetter) error {
	if upCtx.Account == "" {
		return errors.New("error: account is missing from profile")
	}

	// get kubeconfig from connection secret
	nname := types.NamespacedName{Namespace: c.Group, Name: c.Name}
	ctpConfig, err := getter.GetKubeConfig(ctx, nname)
	if controlplane.IsNotFound(err) {
		p.Printfln("Control plane %s not found", nname)
		return nil
	}
	if err != nil {
		return err
	}

	// extract relevant context
	contextName := fmt.Sprintf("%s-%s-%s/%s", upCtx.Account, upCtx.ProfileName, c.Group, c.Name)
	if c.Context != "" {
		contextName = c.Context
	}
	ctpConfig, err = ExtractControlPlaneContext(ctpConfig, ExpectedConnectionSecretContext(upCtx.Account, c.Name), contextName)
	if err != nil {
		return err
	}

	if c.File == "-" {
		ctpConfig.Kind = "Config"
		ctpConfig.APIVersion = "v1"
		bs, err := clientcmd.Write(*ctpConfig)
		if err != nil {
			return err
		}
		p.Printfln(string(bs))
	} else {
		if err := kube.MergeIntoKubeConfig(ctpConfig, c.File, true, kube.VerifyKubeConfig(upCtx.WrapTransport)); err != nil {
			return err
		}
		p.Printfln("Current context set to %s", contextName)
	}

	return nil
}
