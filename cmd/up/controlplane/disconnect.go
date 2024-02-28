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
	"k8s.io/client-go/tools/clientcmd/api"

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

	kcloader, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return err
	}

	cptContext := kcloader.CurrentContext
	if !strings.HasPrefix(cptContext, upboundPrefix) {
		return errors.New("current kube context is not a control plane context")
	}

	target, err := origContext(cptContext)
	if err != nil {
		return err
	}

	if err := switchContext(kcloader, target); err != nil {
		return err
	}

	kcloader.CurrentContext = target
	modifiedCfg, err := removeFromConfig(kcloader, cptContext)
	if err != nil {
		return err
	}

	if err := clientcmd.ModifyConfig(clientcmd.NewDefaultClientConfigLoadingRules(), modifiedCfg, false); err != nil {
		return err
	}

	p.Printfln("Disconnected from control plane %q and switched back to context %q.", cptContext, target)
	return nil
}

func origContext(currentCtx string) (string, error) {
	parts := strings.SplitN(currentCtx, "_", 4)
	if len(parts) != 4 {
		return "", fmt.Errorf(errFmtContextParts, len(parts))
	}
	return parts[len(parts)-1], nil
}

func switchContext(cfg api.Config, target string) error {
	cfg.CurrentContext = target
	return clientcmd.ModifyConfig(clientcmd.NewDefaultClientConfigLoadingRules(), cfg, false)
}

func removeFromConfig(cfg api.Config, contextName string) (api.Config, error) {
	if cfg.CurrentContext == contextName {
		return api.Config{}, fmt.Errorf(errFmtCurrentContext, contextName)
	}

	delete(cfg.AuthInfos, contextName)
	delete(cfg.Clusters, contextName)
	delete(cfg.Contexts, contextName)
	return cfg, nil
}
