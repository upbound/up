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

// Please note: As of March 2023, the `upbound` commands have been disabled.
// We're keeping the code here for now, so they're easily resurrected.
// The upbound commands were meant to support the Upbound self-hosted option.

package upbound

import (
	"context"
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/upbound/up/internal/auth"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/license"
)

const upboundChart = "upbound"

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}
	kongCtx.Bind(&install.Context{
		Kubeconfig: kubeconfig,
		Namespace:  c.Namespace,
	})
	return nil
}

// Cmd contains commands for managing Upbound.
type Cmd struct {
	Install   installCmd   `cmd:"" maturity:"alpha" help:"Install Upbound."`
	Mail      mailCmd      `cmd:"" maturity:"alpha" help:"Run a local mail portal."`
	Uninstall uninstallCmd `cmd:"" maturity:"alpha" help:"Uninstall Upbound."`
	Upgrade   upgradeCmd   `cmd:"" maturity:"alpha" help:"Upgrade Upbound."`

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"UPBOUND_NAMESPACE" default:"upbound-system" help:"Kubernetes namespace for Upbound."`
}

// commonParams are common parameters used across Upbound install and upgrade
// commands.
type commonParams struct {
	LicenseSecretName string `default:"upbound-license" help:"Name of secret that will be populated with license data."`
	SkipLicense       bool   `hidden:"" help:"Skip providing a license for Upbound install."`

	Repo      *url.URL `hidden:"" env:"UPBOUND_REPO" default:"xpkg.upbound.io/upbound" help:"Set repo for Upbound."`
	Registry  *url.URL `hidden:"" env:"UPBOUND_REGISTRY_ENDPOINT" default:"https://xpkg.upbound.io" help:"Set registry for authentication."`
	DMV       *url.URL `hidden:"" env:"UPBOUND_DMV_ENDPOINT" default:"https://dmv.upbound.io" help:"Set DMV for Upbound."`
	OrgID     string   `hidden:"" env:"UPBOUND_ORG_ID" default:"upbound" help:"Set orgID for Upbound."`
	ProductID string   `hidden:"" env:"UPBOUND_PRODUCT_ID" default:"upbound" help:"Set productID for Upbound."`

	KeyVersionOverride string `hidden:"" env:"UPBOUND_KEY_VERSION" help:"Set the key version to use for Upbound install."`
}

// TODO(hasheddan): consider refactoring shared applicator's below into a common
// interface.

// accessKeyApplicator fetches an Upbound access key and signature and
// creates or updates a Secret with its contents.
type accessKeyApplicator struct {
	auth    auth.Provider
	license license.Provider
	secret  *kube.SecretApplicator
}

// newAccessKeyApplicator constructs a new accessKeyApplicator with the passed
// providers.
func newAccessKeyApplicator(auth auth.Provider, license license.Provider, secret *kube.SecretApplicator) *accessKeyApplicator {
	return &accessKeyApplicator{
		auth:    auth,
		license: license,
		secret:  secret,
	}
}

// apply authenticates to the token service with the passed credentials, then
// fetches an access key for the specified user and software version.
func (l *accessKeyApplicator) apply(ctx context.Context, name, ns, version string) error {
	resp, err := l.auth.GetToken(ctx)
	if err != nil {
		return errors.Wrap(err, errGetRegistryToken)
	}
	acc, err := l.license.GetAccessKey(ctx, resp.AccessToken, version)
	if err != nil {
		return errors.Wrap(err, errGetAccessKey)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		StringData: map[string]string{
			defaultSecretAccessKey: acc.AccessKey,
			defaultSecretSignature: acc.Signature,
		},
	}
	// Create access key secret if it does not exist.
	return l.secret.Apply(ctx, ns, secret)
}
