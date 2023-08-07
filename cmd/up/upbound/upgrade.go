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

package upbound

import (
	"context"
	"io"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upterm"
)

const (
	errParseUpgradeParameters = "unable to parse upgrade parameters"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *upgradeCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(insCtx *install.Context, quiet config.QuietFlag) error {
	// id, err := c.prompter.Prompt("License ID", false)
	// if err != nil {
	// 	return err
	// }
	token, err := c.prompter.Prompt("License Key", true)
	if err != nil {
		return err
	}
	c.id = `oauth2accesstoken`
	c.token = token
	kClient, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = kClient
	secret := kube.NewSecretApplicator(kClient)
	c.pullSecret = kube.NewImagePullApplicator(secret)
	dClient, err := dynamic.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.dClient = dClient
	// auth := auth.NewProvider(
	// 	auth.WithBasicAuth(id, token),
	// 	auth.WithEndpoint(c.Registry),
	// 	auth.WithOrgID(c.OrgID),
	// 	auth.WithProductID(c.ProductID),
	// )
	// license := license.NewProvider(
	// 	license.WithEndpoint(c.DMV),
	// 	license.WithOrgID(c.OrgID),
	// 	license.WithProductID(c.ProductID),
	// )
	// c.access = newAccessKeyApplicator(auth, license, secret)
	// c.access = newAccessKeyApplicator(secret, c.id, c.token, c.Registry.String())
	ins, err := helm.NewManager(insCtx.Kubeconfig,
		mxeChart,
		c.Repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithBasicAuth(c.id, c.token),
		helm.IsOCI(),
		helm.WithChart(c.Bundle),
		helm.RollbackOnError(c.Rollback))
	if err != nil {
		return err
	}
	c.mgr = ins
	base := map[string]any{}
	if c.File != nil {
		defer c.File.Close() //nolint:errcheck,gosec
		b, err := io.ReadAll(c.File)
		if err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := yaml.Unmarshal(b, &base); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
		if err := c.File.Close(); err != nil {
			return errors.Wrap(err, errReadParametersFile)
		}
	}
	c.parser = helm.NewParser(base, c.Set)
	c.quiet = quiet
	return nil
}

// upgradeCmd upgrades Upbound.
type upgradeCmd struct {
	mgr        install.Manager
	parser     install.ParameterParser
	prompter   input.Prompter
	pullSecret *kube.ImagePullApplicator
	id         string
	token      string
	kClient    kubernetes.Interface
	dClient    dynamic.Interface
	quiet      config.QuietFlag

	// NOTE(hasheddan): version is currently required for upgrade with OCI image
	// as latest strategy is undetermined.
	Version string `arg:"" help:"Upbound version to upgrade to."`

	Rollback bool `help:"Rollback to previously installed version on failed upgrade."`

	commonParams
	install.CommonParams
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run(insCtx *install.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseUpgradeParameters)
	}

	// Create or update image pull secret.
	if err := c.pullSecret.Apply(ctx, defaultImagePullSecret, insCtx.Namespace, c.id, c.token, c.Registry.String()); err != nil {
		return errors.Wrap(err, errCreateImagePullSecret)
	}

	// Create or update access key secret unless skip license is specified.
	// if !c.SkipLicense {
	// 	keyVersion := c.Version
	// 	if c.KeyVersionOverride != "" {
	// 		keyVersion = c.KeyVersionOverride
	// 	}
	// 	if err := c.access.apply(ctx, c.LicenseSecretName, insCtx.Namespace, keyVersion); err != nil {
	// 		return errors.Wrap(err, errCreateLicenseSecret)
	// 	}
	// }

	if err := c.upgradeUpbound(context.Background(), params); err != nil {
		return err
	}

	if !c.quiet {
		pterm.Info.WithPrefix(upterm.RaisedPrefix).Println("Upbound ready")
	}

	return nil
}

func (c *upgradeCmd) upgradeUpbound(ctx context.Context, params map[string]any) error {
	upgrade := func() error {
		if err := c.mgr.Upgrade(c.Version, params); err != nil {
			return err
		}
		return nil
	}

	if c.quiet {
		return upgrade()
	}

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Upgrading Upbound", 1, 2),
		upterm.CheckmarkSuccessSpinner,
		upgrade,
	); err != nil {
		return err
	}

	// Print Info message to indicate next large step
	spinnerStart, _ := upterm.EyesInfoSpinner.Start(upterm.StepCounter("Starting Upbound", 2, 2))
	spinnerStart.Info()

	ccancel := make(chan bool)
	stopped := make(chan bool)
	// NOTE(tnthornton) we spin off the deployment watching so that we can
	// watch both the custom resource as well as the deployment events at
	// the same time.
	// TODO(hasheddan): consider using DynamicWatch and cancelling via context.
	go watchDeployments(ctx, c.kClient, ccancel, stopped) //nolint:errcheck

	errC, err := kube.DynamicWatch(ctx, c.dClient.Resource(hostclusterGVR), &watcherTimeout, func(u *unstructured.Unstructured) (bool, error) {
		up := resources.Upbound{Unstructured: *u}
		if resource.IsConditionTrue(up.GetCondition(xpv1.TypeReady)) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if err := <-errC; err != nil {
		return err
	}

	ccancel <- true
	close(ccancel)
	<-stopped
	return nil
}
