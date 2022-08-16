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
	"net"
	"time"

	"github.com/alecthomas/kong"
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
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/license"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
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
	localhost              = "127.0.0.1"

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
	errGetRegistryToken       = "failed to acquire auth token"
	errGetAccessKey           = "failed to acquire access key"
	errCreateImagePullSecret  = "failed to create image pull secret"
	errCreateLicenseSecret    = "failed to create license secret"
	errCreateNamespace        = "failed to create namespace"
	errTimoutExternalIP       = "timed out waiting for externalIP to resolve"
	errUpdateConfig           = "unable to update config"
)

// BeforeApply sets default values in login before assignment and validation.
func (c *installCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(insCtx *install.Context, kongCtx *kong.Context, quiet config.QuietFlag) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

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
	c.quiet = quiet
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
	quiet      config.QuietFlag

	Version string `arg:"" help:"Upbound version to install."`

	commonParams
	install.CommonParams

	Flags upbound.Flags `embed:""`
}

// Run executes the install command.
func (c *installCmd) Run(insCtx *install.Context, upCtx *upbound.Context) error {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}

	if err := c.applySecrets(ctx, insCtx.Namespace); err != nil {
		return err
	}

	if err := c.installUpbound(context.Background(), insCtx.Kubeconfig, params); err != nil {
		return err
	}

	if !c.quiet {
		spinnerIngress, _ := checkmarkSuccessSpinner.Start(stepCounter("Gathering ingress information", 5, 5))
		// Sleep for 1s to ensure pterm has enough time for 1 spin. Without this
		// line, the operation can complete too quick resulting in two lines
		// written for the "Gathering" spinner.
		time.Sleep(1 * time.Second)
		ipAddress, err := c.getExternalIP(params)
		if err != nil {
			return err
		}
		spinnerIngress.Success()

		pterm.Info.WithPrefix(raisedPrefix).Println("Upbound ready")
		time.Sleep(2 * time.Second)

		outputConnectingInfo(ipAddress, hostNames)
	}

	return updateProfile(upCtx)
}

func (c *installCmd) applySecrets(ctx context.Context, namespace string) error { //nolint:gocyclo
	createNs := func() error {
		_, err := c.kClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{})
		if err != nil && !kerrors.IsAlreadyExists(err) {
			return errors.Wrap(err, errCreateNamespace)
		}
		return nil
	}
	creatPullSecret := func() error {
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
	}

	if c.quiet {
		if err := createNs(); err != nil {
			return err
		}
		return creatPullSecret()
	}

	if err := wrapWithSuccessSpinner(
		stepCounter(fmt.Sprintf("Creating namespace %s", namespace), 1, 5),
		checkmarkSuccessSpinner,
		createNs,
	); err != nil {
		return err
	}

	if err := wrapWithSuccessSpinner(
		stepCounter(fmt.Sprintf("Creating secret %s", defaultImagePullSecret), 2, 5),
		checkmarkSuccessSpinner,
		creatPullSecret,
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

func (c *installCmd) installUpbound(ctx context.Context, kubeconfig *rest.Config, params map[string]any) error {
	install := func() error {
		if err := c.mgr.Install(c.Version, params); err != nil {
			return err
		}
		return nil
	}

	if c.quiet {
		return install()
	}

	if err := wrapWithSuccessSpinner(
		stepCounter("Initializing Upbound", 3, 5),
		checkmarkSuccessSpinner,
		install,
	); err != nil {
		return err
	}

	// Print Info message to indicate next large step
	spinnerStart, _ := eyesInfoSpinner.Start(stepCounter("Starting Upbound", 4, 5))
	spinnerStart.Info()

	watchCtx, cancel := context.WithTimeout(ctx, time.Duration(watcherTimeout*int64(time.Second)))
	defer cancel()
	ccancel := make(chan bool)
	stopped := make(chan bool)
	// NOTE(tnthornton) we spin off the deployment watching so that we can
	// watch both the custom resource as well as the deployment events at
	// the same time.
	go watchDeployments(watchCtx, c.kClient, ccancel, stopped) //nolint:errcheck

	if err := watchCustomResource(watchCtx, upboundGVR, kubeconfig); err != nil {
		return err
	}

	ccancel <- true
	close(ccancel)
	<-stopped
	return nil
}

// getExternalIP returns the externalIP of the Upbound installation. At its
// core it's doing two things:
//  1. If the provider is not specified, the kind install is assumed which means
//     localhost is assumed.
//  2. If the provider is specified, the externalIP is derived from the
//     ingress-nginx-controller.
//
// NOTE(tnthornton) this function is a temporary measure to calculate the
// externalIP for the Upbound install until the Upbound Custom Resource exposes
// this information on its status.externalIP field.
func (c *installCmd) getExternalIP(params map[string]any) (string, error) { //nolint:gocyclo
	provider, ok := params["provider"]
	if !ok {
		// default provider (kind) is used, return localhost
		return localhost, nil
	}

	// retry until we have an externalIP or timeout
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)

	for {
		select {
		case <-timeout:
			return "", errors.New(errTimoutExternalIP)
		case <-ticker.C:
			svc, err := c.kClient.
				CoreV1().
				Services("ingress-nginx").
				Get(context.Background(), "ingress-nginx-controller", metav1.GetOptions{})
			if err != nil {
				return "", err
			}

			lbs := svc.Status.LoadBalancer.Ingress
			// if ELB IP is still empty, skip and retry
			if len(lbs) < 1 {
				continue
			}

			switch provider {
			case "aws":
				record := lbs[0].Hostname
				ips, err := net.LookupIP(record)
				if err != nil {
					// NOTE(tnthornton) we explicitly ignore the error here to
					// force a retry. Most commonly an error will occur when
					// DNS has yet to propagate.
					continue
				}
				if len(ips) >= 1 {
					return ips[0].String(), nil
				}
			default:
				ip := lbs[0].IP
				if ip != "" {
					return ip, nil
				}
			}
		}
	}
}

func updateProfile(upCtx *upbound.Context) error {
	// apply selfhosted config
	profileName, config := getSelfHostedProfile(resources.Domain)
	if err := upCtx.Cfg.AddOrUpdateUpboundProfile(profileName, config); err != nil {
		return err
	}

	// switch default profile to selfhosted
	if err := upCtx.Cfg.SetDefaultUpboundProfile(profileName); err != nil {
		return err
	}

	return errors.Wrap(upCtx.CfgSrc.UpdateConfig(upCtx.Cfg), errUpdateConfig)
}
