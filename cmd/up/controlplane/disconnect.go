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
	"errors"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	errFmtCurrentContext = "context %q is currently in use"
	errFmtContextParts   = "given context does not have the correct number of parts, expected: 4, got: %d"
)

// AfterApply sets default values in command after assignment and validation.
func (c *disconnectCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kongCtx.Bind(pterm.DefaultTable.WithWriter(kongCtx.Stdout).WithSeparator("   "))

	return nil
}

// getCmd gets a single control plane in an account on Upbound.
type disconnectCmd struct{}

// Run executes the get command.
func (c *disconnectCmd) Run(printer upterm.ObjectPrinter, p pterm.TextPrinter, upCtx *upbound.Context) error {
	if upCtx.Account == "" {
		return errors.New("error: account is missing from profile")
	}

	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return err
	}

	kubeConfig, err = switchToOrigContext(kubeConfig)
	if err != nil {
		return err
	}

	if err := clientcmd.ModifyConfig(clientcmd.NewDefaultClientConfigLoadingRules(), kubeConfig, false); err != nil {
		return err
	}

	p.Printfln("Switched back to context %q.", kubeConfig.CurrentContext)

	return nil
}

func switchToOrigContext(kubeConfig clientcmdapi.Config) (clientcmdapi.Config, error) {
	if !strings.HasPrefix(kubeConfig.CurrentContext, upboundPrefix) {
		return clientcmdapi.Config{}, errors.New("current kube context is not a control plane context")
	}

	kubeConfig = *kubeConfig.DeepCopy()

	cptContext := kubeConfig.CurrentContext
	orig, err := origContext(cptContext)
	if err != nil {
		return clientcmdapi.Config{}, err
	}

	// switch to the original context and remove ctp context, cluster and auth.
	kubeConfig.CurrentContext = orig
	delete(kubeConfig.AuthInfos, cptContext)
	delete(kubeConfig.Clusters, cptContext)
	delete(kubeConfig.Contexts, cptContext)

	return kubeConfig, nil
}

func origContext(currentCtx string) (string, error) {
	parts := strings.SplitN(currentCtx, "_", 4)
	if len(parts) != 4 {
		return "", fmt.Errorf(errFmtContextParts, len(parts))
	}
	return parts[len(parts)-1], nil
}
