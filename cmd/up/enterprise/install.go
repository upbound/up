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
	"io"
	"net/url"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/cmd/create"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/auth"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/license"
)

const (
	defaultTimeout         = 30 * time.Second
	defaultSecretAccessKey = "access_key"
	defaultSecretSignature = "signature"
	defaultImagePullSecret = "enterprise-pull-secret"

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
	errGetRegistryToken       = "failed to acquire auth token"
	errGetAccessKey           = "failed to acquire access key"
	errCreateImagePullSecret  = "failed to create image pull secret"
	errCreateLicenseSecret    = "failed to create license secret"
	errCreateNamespace        = "failed to create namespace"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *installCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(insCtx *install.Context) error {
	id, err := c.prompter.Prompt("License ID", false)
	if err != nil {
		return err
	}
	token, err := c.prompter.Prompt("Token", true)
	if err != nil {
		return err
	}
	c.id = id
	c.token = token

	c.auth = auth.NewProvider(
		auth.WithBasicAuth(id, token),
		auth.WithEndpoint(c.Registry),
		auth.WithOrgID(c.OrgID),
		auth.WithProductID(c.ProductID),
	)

	c.license = license.NewProvider(
		license.WithEndpoint(c.DMV),
		license.WithOrgID(c.OrgID),
		license.WithProductID(c.ProductID),
	)

	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		enterpriseChart,
		c.Repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithBasicAuth(id, token),
		helm.IsOCI(),
		helm.WithChart(c.Bundle))
	if err != nil {
		return err
	}
	c.mgr = mgr
	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	base := map[string]interface{}{}
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
	return nil
}

// installCmd installs enterprise.
type installCmd struct {
	mgr      install.Manager
	parser   install.ParameterParser
	kClient  kubernetes.Interface
	prompter input.Prompter
	auth     auth.Provider
	license  license.Provider
	id       string
	token    string

	Version string `arg:"" help:"Enterprise version to install."`

	LicenseSecretName string `default:"upbound-enterprise-license" help:"Name of secret that will be populated with license data."`
	SkipLicense       bool   `hidden:"" help:"Skip providing a license for enteprise install."`

	Repo      *url.URL `hidden:"" env:"ENTERPRISE_REPO" default:"registry.upbound.io/enterprise" help:"Set repo for enterprise."`
	Registry  *url.URL `hidden:"" env:"ENTERPRISE_REGISTRY_ENDPOINT" default:"https://registry.upbound.io" help:"Set registry for authentication."`
	DMV       *url.URL `hidden:"" env:"ENTERPRISE_DMV_ENDPOINT" default:"https://dmv.upbound.io" help:"Set dmv for enterprise."`
	OrgID     string   `hidden:"" env:"ENTERPRISE_ORG_ID" default:"enterprise" help:"Set orgID for enterprise."`
	ProductID string   `hidden:"" env:"ENTERPRISE_PRODUCT_ID" default:"enterprise" help:"Set productID for enterprise."`

	KeyVersionOverride string `hidden:"" env:"ENTERPRISE_KEY_VERSION" help:"Set the key version to use for enterprise install."`

	install.CommonParams
}

// Run executes the install command.
func (c *installCmd) Run(insCtx *install.Context) error {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}

	// Create namespace if it does not exist.
	_, err = c.kClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: insCtx.Namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, errCreateNamespace)
	}

	// Create or update image pull secret.
	if err := c.applyImagePullSecret(ctx, insCtx.Namespace); err != nil {
		return errors.Wrap(err, errCreateImagePullSecret)
	}

	// Create or update access key secret unless skip license is specified.
	if !c.SkipLicense {
		if err := c.applyLicense(ctx, insCtx.Namespace); err != nil {
			return errors.Wrap(err, errCreateLicenseSecret)
		}
	}

	err = c.mgr.Install(c.Version, params)
	if err != nil {
		return err
	}
	return err
}

func (c *installCmd) applyLicense(ctx context.Context, ns string) error {
	resp, err := c.auth.GetToken(ctx)
	if err != nil {
		return errors.Wrap(err, errGetRegistryToken)
	}

	v := c.Version
	if c.KeyVersionOverride != "" {
		v = c.KeyVersionOverride
	}

	acc, err := c.license.GetAccessKey(ctx, resp.AccessToken, v)
	if err != nil {
		return errors.Wrap(err, errGetAccessKey)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.LicenseSecretName,
		},
		StringData: map[string]string{
			defaultSecretAccessKey: acc.AccessKey,
			defaultSecretSignature: acc.Signature,
		},
	}

	// Create license secret if it does not exist.
	_, err = c.kClient.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && kerrors.IsAlreadyExists(err) {
		_, err = c.kClient.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{})
	}

	return err
}

func (c *installCmd) applyImagePullSecret(ctx context.Context, ns string) error {
	regAuth := &create.DockerConfigJSON{
		Auths: map[string]create.DockerConfigEntry{
			c.Registry.String(): {
				Username: c.id,
				Password: c.token,
				Auth:     encodeDockerConfigFieldAuth(c.id, c.token),
			},
		},
	}
	regAuthJSON, err := json.Marshal(regAuth)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultImagePullSecret,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: regAuthJSON,
		},
	}
	_, err = c.kClient.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && kerrors.IsAlreadyExists(err) {
		_, err = c.kClient.CoreV1().Secrets(ns).Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}

// encodeDockerConfigFieldAuth returns base64 encoding of the username and
// password string
// NOTE(hasheddan): this function comes directly from kubectl
// https://github.com/kubernetes/kubectl/blob/0f88fc6b598b7e883a391a477215afb080ec7733/pkg/cmd/create/create_secret_docker.go#L305
func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}
