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

package enterprise

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/cmd/create"

	"github.com/upbound/up/internal/auth"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/license"
)

const enterpriseChart = "enterprise"

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

// Cmd contains commands for managing enterprise.
type Cmd struct {
	Install   installCmd   `cmd:"" group:"enterprise" help:"Install enterprise."`
	Mail      mailCmd      `cmd:"" group:"enterprise" help:"[EXPERIMENTAL] Run a local mail portal."`
	Uninstall uninstallCmd `cmd:"" group:"enterprise" help:"Uninstall enterprise."`
	Upgrade   upgradeCmd   `cmd:"" group:"enterprise" help:"Upgrade enterprise."`

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"ENTERPRISE_NAMESPACE" default:"upbound-enterprise" help:"Kubernetes namespace for enterprise."`
}

// commonParams are common parameters used across enterprise install and upgrade
// commands.
type commonParams struct {
	LicenseSecretName string `default:"upbound-enterprise-license" help:"Name of secret that will be populated with license data."`
	SkipLicense       bool   `hidden:"" help:"Skip providing a license for enterprise install."`

	Repo      *url.URL `hidden:"" env:"ENTERPRISE_REPO" default:"registry.upbound.io/enterprise" help:"Set repo for enterprise."`
	Registry  *url.URL `hidden:"" env:"ENTERPRISE_REGISTRY_ENDPOINT" default:"https://registry.upbound.io" help:"Set registry for authentication."`
	DMV       *url.URL `hidden:"" env:"ENTERPRISE_DMV_ENDPOINT" default:"https://dmv.upbound.io" help:"Set dmv for enterprise."`
	OrgID     string   `hidden:"" env:"ENTERPRISE_ORG_ID" default:"enterprise" help:"Set orgID for enterprise."`
	ProductID string   `hidden:"" env:"ENTERPRISE_PRODUCT_ID" default:"enterprise" help:"Set productID for enterprise."`

	KeyVersionOverride string `hidden:"" env:"ENTERPRISE_KEY_VERSION" help:"Set the key version to use for enterprise install."`
}

// TODO(hasheddan): consider refactoring shared applicator's below into a common
// interface.

// accessKeyApplicator fetches an enterprise access key and signature and
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

// imagePullApplicator constructs and creates or updates an image pull Secret.
type imagePullApplicator struct {
	secret *kube.SecretApplicator
}

// newImagePullApplicator constructs a new imagePullApplicator with the passed
// SecretApplicator.
func newImagePullApplicator(secret *kube.SecretApplicator) *imagePullApplicator {
	return &imagePullApplicator{
		secret: secret,
	}
}

// apply constructs an DockerConfig image pull Secret with the provided registry
// and credentials.
func (i *imagePullApplicator) apply(ctx context.Context, name, ns, user, pass, registry string) error {
	regAuth := &create.DockerConfigJSON{
		Auths: map[string]create.DockerConfigEntry{
			registry: {
				Username: user,
				Password: pass,
				Auth:     encodeDockerConfigFieldAuth(user, pass),
			},
		},
	}
	regAuthJSON, err := json.Marshal(regAuth)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: regAuthJSON,
		},
	}
	// Create image pull secret if it does not exist.
	return i.secret.Apply(ctx, ns, secret)
}

// encodeDockerConfigFieldAuth returns base64 encoding of the username and
// password string
// NOTE(hasheddan): this function comes directly from kubectl
// https://github.com/kubernetes/kubectl/blob/0f88fc6b598b7e883a391a477215afb080ec7733/pkg/cmd/create/create_secret_docker.go#L305
func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
