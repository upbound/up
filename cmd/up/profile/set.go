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

package profile

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"

	_ "embed"
)

const (
	errSetProfile   = "unable to set profile"
	errUpdateConfig = "unable to update config file"
	errNoSpace      = "cannot find Spaces in the Kubernetes cluster. Run 'up space init' to install Spaces."
	errKubeContact  = "unable to check for Spaces on Kubernetes cluster"
)

type setCmd struct {
	Space spaceCmd `cmd:"" help:"Set an Upbound Profile for use with a Space."`
}

type spaceCmd struct {
	Kube upbound.KubeFlags `embed:""`

	getClient func() (kubernetes.Interface, error)
}

//go:embed space_help.txt
var spaceCmdHelp string

func (c *spaceCmd) Help() string {
	return spaceCmdHelp
}

func (c *spaceCmd) AfterApply(kongCtx *kong.Context) error {
	return c.Kube.AfterApply()
}

func (c *spaceCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
	setDefault := false

	// If profile name was not provided and no default exists, set name to
	// the default, and set this profile as the default profile.
	if upCtx.ProfileName == "" {
		upCtx.ProfileName = profile.DefaultName
		setDefault = true
	}

	prof := profile.Profile{
		Account:     upCtx.Account,
		Type:        profile.Space,
		Kubeconfig:  c.Kube.Kubeconfig,
		KubeContext: c.Kube.GetContext(),
		// Carry over existing config.
		BaseConfig: upCtx.Profile.BaseConfig,
	}

	installed, err := c.checkForSpaces(ctx)
	if err != nil {
		return err
	}
	if !installed {
		return errors.New(errNoSpace)
	}

	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(upCtx.ProfileName, prof); err != nil {
		return errors.Wrap(err, errSetProfile)
	}

	if setDefault {
		if err := upCtx.Cfg.SetDefaultUpboundProfile(upCtx.ProfileName); err != nil {
			return errors.Wrap(err, errSetProfile)
		}
	}

	if err := upCtx.CfgSrc.UpdateConfig(upCtx.Cfg); err != nil {
		return errors.Wrap(err, errUpdateConfig)
	}

	kubeconfigLocation := "default kubeconfig"
	if prof.Kubeconfig != "" {
		kubeconfigLocation = fmt.Sprintf("kubeconfig at %q", prof.Kubeconfig)
	}
	p.Printf("Profile %q updated to use Kubernetes context %q from the %s. Defaulting to group %q.", upCtx.ProfileName, prof.KubeContext, kubeconfigLocation, c.Kube.Namespace())
	if setDefault {
		p.Print(" and selected as the default profile")
	}
	p.Println()

	return nil
}

func (c *spaceCmd) checkForSpaces(ctx context.Context) (bool, error) {
	kubeconfig := c.Kube.GetConfig()
	var kClient kubernetes.Interface
	var err error
	if c.getClient != nil {
		kClient, err = c.getClient()
	} else {
		kClient, err = kubernetes.NewForConfig(kubeconfig)
	}
	if err != nil {
		return false, err
	}
	if _, err := kClient.AppsV1().Deployments("upbound-system").Get(ctx, "mxe-controller", metav1.GetOptions{}); kerrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, errKubeContact)
	}

	return true, nil
}
