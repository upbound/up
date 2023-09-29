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

package space

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	errParseUpgradeParameters      = "unable to parse upgrade parameters"
	errFailedGettingCurrentVersion = "failed to retrieve current version"
	errInvalidVersionFmt           = "invalid version %q"
	errAborted                     = "aborted"
)

// upgradeCmd upgrades Upbound.
type upgradeCmd struct {
	Upbound  upbound.Flags           `embed:""`
	Kube     upbound.KubeFlags       `embed:""`
	Registry authorizedRegistryFlags `embed:""`
	install.CommonParams

	// NOTE(hasheddan): version is currently required for upgrade with OCI image
	// as latest strategy is undetermined.
	Version string `arg:"" help:"Upbound Spaces version to upgrade to."`

	Rollback bool `help:"Rollback to previously installed version on failed upgrade."`

	helmMgr    install.Manager
	parser     install.ParameterParser
	prompter   input.Prompter
	pullSecret *kube.ImagePullApplicator
	kClient    kubernetes.Interface
	quiet      config.QuietFlag
	oldVersion string
	downgrade  bool
}

// BeforeApply sets default values in login before assignment and validation.
func (c *upgradeCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *upgradeCmd) AfterApply(quiet config.QuietFlag) error { //nolint:gocyclo
	if err := c.Kube.AfterApply(); err != nil {
		return err
	}
	if err := c.Registry.AfterApply(); err != nil {
		return err
	}

	// NOTE(tnthornton) we currently only have support for stylized output.
	pterm.EnableStyling()
	upterm.DefaultObjPrinter.Pretty = true

	upCtx, err := upbound.NewFromFlags(c.Upbound)
	if err != nil {
		return err
	}

	kubeconfig, err := c.getKubeconfig(upCtx)
	if err != nil {
		return err
	}

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = kClient
	secret := kube.NewSecretApplicator(kClient)
	c.pullSecret = kube.NewImagePullApplicator(secret)
	ins, err := helm.NewManager(kubeconfig,
		spacesChart,
		c.Registry.Repository,
		helm.WithNamespace(ns),
		helm.WithBasicAuth(c.Registry.Username, c.Registry.Password),
		helm.IsOCI(),
		helm.WithChart(c.Bundle),
		helm.RollbackOnError(c.Rollback),
		helm.Wait())
	if err != nil {
		return err
	}
	c.helmMgr = ins
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
	c.oldVersion, err = ins.GetCurrentVersion()
	if err != nil {
		return errors.Wrap(err, errFailedGettingCurrentVersion)
	}

	// validate versions
	if c.Bundle == nil {
		from, err := semver.Parse(c.oldVersion)
		if err != nil {
			return errors.Wrapf(err, errInvalidVersionFmt, c.oldVersion)
		}
		to, err := semver.Parse(strings.TrimPrefix(c.Version, "v"))
		if err != nil {
			return errors.Wrapf(err, errInvalidVersionFmt, c.Version)
		}
		c.downgrade = from.GT(to)

		if err := c.validateVersions(from, to); err != nil {
			return err
		}
	}

	return nil
}

// getKubeconfig returns the kubeconfig from flags if provided, otherwise the
// kubeconfig from the active profile.
func (c *upgradeCmd) getKubeconfig(upCtx *upbound.Context) (*rest.Config, error) {
	if c.Kube.Kubeconfig != "" || c.Kube.Context != "" {
		return c.Kube.GetConfig(), nil
	}
	if !upCtx.Profile.IsSpace() {
		return nil, fmt.Errorf("upgrade is not supported for non-space profile %q", upCtx.ProfileName)
	}
	return upCtx.Profile.GetKubeClientConfig()
}

// Run executes the upgrade command.
func (c *upgradeCmd) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseUpgradeParameters)
	}
	overrideRegistry(c.Registry.Repository.String(), params)

	// Create or update image pull secret.
	if err := c.pullSecret.Apply(ctx, defaultImagePullSecret, ns, c.Registry.Username, c.Registry.Password, c.Registry.Endpoint.String()); err != nil {
		return errors.Wrap(err, errCreateImagePullSecret)
	}

	if err := c.upgradeUpbound(params); err != nil {
		return err
	}

	return nil
}

func upgradeVersionBounds(_ string, ch *chart.Chart) error {
	return checkVersion(fmt.Sprintf("unsupported target chart version %s", ch.Metadata.Version), upgradeVersionConstraints, ch.Metadata.Version)
}

func upgradeFromVersionBounds(from string, ch *chart.Chart) error {
	return checkVersion(fmt.Sprintf("unsupported installed chart version %s", ch.Metadata.Version), upgradeFromVersionConstraints, from)
}

func upgradeUpVersionBounds(_ string, ch *chart.Chart) error {
	return upVersionBounds(ch)
}

func (c *upgradeCmd) upgradeUpbound(params map[string]any) error {
	version := strings.TrimPrefix(c.Version, "v")
	upgrade := func() error {
		if err := c.helmMgr.Upgrade(version, params, upgradeUpVersionBounds, upgradeFromVersionBounds, upgradeVersionBounds); err != nil {
			return err
		}
		return nil
	}

	verb := "Upgrading"
	if c.downgrade {
		verb = "Downgrading"
	}

	if err := upterm.WrapWithSuccessSpinner(
		fmt.Sprintf("%s Space from v%s to v%s", verb, c.oldVersion, version),
		upterm.CheckmarkSuccessSpinner,
		upgrade,
	); err != nil {
		fmt.Println()
		fmt.Println()
		return err
	}

	return nil
}

func (c *upgradeCmd) validateVersions(from, to semver.Version) error {
	switch {
	case c.downgrade:
		if err := warnAndConfirm("Downgrades are not supported."); err != nil {
			return err
		}
	case to.Major > from.Major:
		if err := warnAndConfirm("Upgrades to a new major version are only supported for explicitly documented releases."); err != nil {
			return err
		}
	case to.Minor > from.Minor+1:
		if err := warnAndConfirm("Upgrades which skip a minor version are not supported."); err != nil {
			return err
		}
	default:
	}

	return nil
}

func warnAndConfirm(warning string, args ...any) error {
	pterm.Println()
	pterm.Warning.Printfln(warning, args...)
	pterm.Println() // Blank line
	confirm := pterm.DefaultInteractiveConfirm
	confirm.DefaultText = "Are you sure you want to proceed?"
	result, _ := confirm.Show()
	pterm.Println() // Blank line
	if !result {
		return errors.New(errAborted)
	}
	return nil
}
