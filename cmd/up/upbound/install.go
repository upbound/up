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
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/internal/auth"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/license"
)

var (
	watcherTimeout int64 = 600

	upboundGVR = schema.GroupVersionResource{
		Group:    upboundGroup,
		Version:  upboundVersion,
		Resource: upboundResourcePlural,
	}
)

const (
	defaultTimeout = 30 * time.Second

	defaultSecretAccessKey = "access_key"
	defaultSecretSignature = "signature"
	defaultImagePullSecret = "upbound-pull-secret"

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
	token, err := c.prompter.Prompt("License Key", true)
	if err != nil {
		return err
	}
	c.id = id
	c.token = token
	client, err := kubernetes.NewForConfig(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	secret := kube.NewSecretApplicator(client)
	c.pullSecret = newImagePullApplicator(secret)
	auth := auth.NewProvider(
		auth.WithBasicAuth(c.id, c.token),
		auth.WithEndpoint(c.Registry),
		auth.WithOrgID(c.OrgID),
		auth.WithProductID(c.ProductID),
	)
	license := license.NewProvider(
		license.WithEndpoint(c.DMV),
		license.WithOrgID(c.OrgID),
		license.WithProductID(c.ProductID),
	)
	c.access = newAccessKeyApplicator(auth, license, secret)
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		upboundChart,
		c.Repo,
		helm.WithNamespace(insCtx.Namespace),
		helm.WithBasicAuth(c.id, c.token),
		helm.IsOCI(),
		helm.WithChart(c.Bundle))
	if err != nil {
		return err
	}
	c.mgr = mgr
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
	return nil
}

// installCmd installs Upbound.
type installCmd struct {
	mgr        install.Manager
	parser     install.ParameterParser
	kClient    kubernetes.Interface
	prompter   input.Prompter
	access     *accessKeyApplicator
	pullSecret *imagePullApplicator
	id         string
	token      string

	Version string `arg:"" help:"Upbound version to install."`

	commonParams
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

	if err := c.applySecrets(ctx, insCtx.Namespace); err != nil {
		return err
	}

	if err := c.installUpbound(ctx, insCtx.Kubeconfig, params); err != nil {
		return err
	}

	spinnerIngress, _ := checkmarkSuccessSpinner.Start("Gathering ingress information")
	spinnerIngress.Success()
	time.Sleep(time.Second * 1)

	pterm.Info.WithPrefix(raisedPrefix).Println("Upbound ready")
	time.Sleep(2 * time.Second)

	outputConnectingInfo(ipAddress, hostNames)

	return err
}

func (c *installCmd) applySecrets(ctx context.Context, namespace string) error {
	if err := wrapWithSuccessSpinner(
		fmt.Sprintf("Creating namespace %s", namespace),
		checkmarkSuccessSpinner,
		func() error {
			_, err := c.kClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}, metav1.CreateOptions{})
			if err != nil && !kerrors.IsAlreadyExists(err) {
				return errors.Wrap(err, errCreateNamespace)
			}
			return nil
		},
	); err != nil {
		return err
	}

	if err := wrapWithSuccessSpinner(
		fmt.Sprintf("Creating secret %s", defaultImagePullSecret),
		checkmarkSuccessSpinner,
		func() error {
			if err := c.pullSecret.apply(
				ctx,
				defaultImagePullSecret,
				namespace,
				c.id,
				c.token,
				c.Registry.String(),
			); err != nil {
				return errors.Wrap(err, errCreateImagePullSecret)
			}
			return nil
		},
	); err != nil {
		return err
	}

	// Create or update access key secret unless skip license is specified.
	if !c.SkipLicense {
		keyVersion := c.Version
		if c.KeyVersionOverride != "" {
			keyVersion = c.KeyVersionOverride
		}
		if err := c.access.apply(ctx, c.LicenseSecretName, namespace, keyVersion); err != nil {
			return errors.Wrap(err, errCreateLicenseSecret)
		}
	}
	return nil
}

func (c *installCmd) installUpbound(_ context.Context, kubeconfig *rest.Config, params map[string]any) error {
	if err := wrapWithSuccessSpinner(
		"Initializing Upbound",
		checkmarkSuccessSpinner,
		func() error {
			if err := c.mgr.Install(c.Version, params); err != nil {
				return err
			}
			return nil
		},
	); err != nil {
		return err
	}

	// Print Info message to indicate next large step
	spinnerStart, _ := eyesInfoSpinner.Start("Starting Upbound")
	spinnerStart.Info()

	watchCtx := context.Background()
	ccancel := make(chan bool)
	stopped := make(chan bool)
	// NOTE(tnthornton) we spin off the deployment watching so that we can
	// watch both the custom resource as well as the deployment events at
	// the same time.
	go watchDeployments(context.Background(), c.kClient, ccancel, stopped) //nolint:errcheck

	if err := watchCustomResource(watchCtx, upboundGVR, kubeconfig); err != nil {
		return err
	}

	ccancel <- true
	close(ccancel)
	<-stopped
	return nil
}
