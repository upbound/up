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

package space

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/google/uuid"
	"github.com/pterm/pterm"
	"helm.sh/helm/v3/pkg/storage/driver"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	sdkerrs "github.com/upbound/up-sdk-go/errors"
	"github.com/upbound/up-sdk-go/service/robots"
	"github.com/upbound/up-sdk-go/service/spaces"
	"github.com/upbound/up-sdk-go/service/tokens"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

type detachCmd struct {
	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`
}

func (c *detachCmd) AfterApply(kongCtx *kong.Context) error {
	registryURL, err := url.Parse(agentRegistry)
	if err != nil {
		return err
	}

	if err := c.Kube.AfterApply(); err != nil {
		return err
	}

	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)
	kongCtx.Bind(tokens.NewClient(cfg))
	kongCtx.Bind(robots.NewClient(cfg))
	kongCtx.Bind(spaces.NewClient(cfg))

	kubeconfig := c.Kube.GetConfig()

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	kongCtx.Bind(kClient)

	with := []helm.InstallerModifierFn{
		helm.WithNamespace(agentNs),
		helm.IsOCI(),
	}

	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		registryURL,
		with...,
	)
	if err != nil {
		return err
	}
	kongCtx.Bind(mgr)

	return nil
}

// Run executes the install command.
func (c *detachCmd) Run(ctx context.Context, kClient *kubernetes.Clientset, mgr *helm.Installer, sc *spaces.Client, rc *robots.Client, tc *tokens.Client) (rErr error) {
	detachSpinner, err := upterm.CheckmarkSuccessSpinner.Start("Removing agent from Space...")
	if err != nil {
		return err
	}
	defer func() {
		if rErr != nil {
			detachSpinner.Fail(rErr)
		}
	}()
	t, err := getTokenSecret(ctx, kClient, agentNs, agentSecret)
	if kerrors.IsNotFound(err) {
		detachSpinner.InfoPrinter.Printfln("Space is not attached, please run %q to attach it first", "up space attach")
	}
	if err != nil {
		return err
	}
	if err := c.deleteAgentToken(ctx, detachSpinner.InfoPrinter, tc, t.Data); err != nil {
		return err
	}
	if err := c.deleteGeneratedSpace(ctx, detachSpinner.InfoPrinter, sc, t.Data); err != nil {
		return err
	}
	detachSpinner.InfoPrinter.Println("Uninstalling connect agent...")
	if err := mgr.Uninstall(); err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return err
	}
	if err := deleteTokenSecret(ctx, detachSpinner.InfoPrinter, kClient, agentNs, agentSecret); err != nil {
		return err
	}
	detachSpinner.Success("Space detached")
	return nil
}

func (c *detachCmd) deleteAgentToken(ctx context.Context, p pterm.TextPrinter, tc *tokens.Client, data map[string][]byte) error {
	if v, ok := data[keyTokenID]; ok {
		tid := uuid.UUID(v)
		p.Printfln("Deleting agent Token %q...", tid)
		if err := tc.Delete(ctx, tid); err != nil && !sdkerrs.IsNotFound(err) {
			return err
		}
		p.Printfln("Token %q deleted", tid)
	}
	return nil
}

func (c *detachCmd) deleteGeneratedSpace(ctx context.Context, p pterm.TextPrinter, sc *spaces.Client, data map[string][]byte) error {
	if v, ok := data[keyGeneratedSpace]; ok {
		sid := string(v)
		p.Println("Deleting generated Space %q...", sid)
		parts := strings.Split(sid, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("unable to determine organization and space name from %q", sid)
		}
		ns, name := parts[0], parts[1]
		if err := sc.Delete(ctx, ns, name, nil); err != nil && !kerrors.IsNotFound(err) {
			return err
		}
		p.Printfln("Space %q deleted", sid)
	}
	return nil
}
