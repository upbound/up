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
	"io"
	"net/url"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
	errGetRegistryToken       = "failed to acquire auth token"
	errGetAccessKey           = "failed to acquire access key"
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

	Version string `arg:"" help:"Enterprise version to install."`

	LicenseSecretName string `default:"upbound-enterprise-license" help:"Name of secret that will be populated with license data."`

	Repo      *url.URL `hidden:"" env:"ENTERPRISE_REPO" default:"registry.upbound.io/enterprise" help:"Set repo for enterprise."`
	Registry  *url.URL `hidden:"" env:"ENTERPRISE_REGISTRY_ENDPOINT" default:"https://registry.upbound.io" help:"Set registry for authentication."`
	DMV       *url.URL `hidden:"" env:"ENTERPRISE_DMV_ENDPOINT" default:"http://localhost:8080" help:"Set dmv for enterprise."`
	OrgID     string   `hidden:"" env:"ENTERPRISE_ORG_ID" default:"enterprise-dev" help:"Set orgID for enterprise."`
	ProductID string   `hidden:"" env:"ENTERPRISE_PRODUCT_ID" default:"enterprise" help:"Set productID for enterprise."`

	install.CommonParams
}

// Run executes the install command.
func (c *installCmd) Run(insCtx *install.Context) error {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := c.auth.GetToken(ctx)
	if err != nil {
		return errors.Wrap(err, errGetRegistryToken)
	}

	acc, err := c.license.GetAccessKey(ctx, resp.AccessToken, c.Version)
	if err != nil {
		return errors.Wrap(err, errGetAccessKey)
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
	_, err = c.kClient.CoreV1().Secrets(insCtx.Namespace).Create(
		ctx,
		secret,
		metav1.CreateOptions{},
	)
	if err != nil && kerrors.IsAlreadyExists(err) {
		if _, err = c.kClient.CoreV1().Secrets(insCtx.Namespace).Update(
			ctx,
			secret,
			metav1.UpdateOptions{},
		); err != nil {
			return errors.Wrap(err, errCreateLicenseSecret)
		}
	}

	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	err = c.mgr.Install(c.Version, params)
	if err != nil {
		return err
	}
	return err
}
