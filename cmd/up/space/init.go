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

package space

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up/cmd/up/space/prerequistes"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/input"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	hcGroup          = "internal.spaces.upbound.io"
	hcVersion        = "v1alpha1"
	hcKind           = "XHostCluster"
	hcResourcePlural = "hostclusters"
)

var (
	watcherTimeout int64 = 600

	hostclusterGVR = schema.GroupVersionResource{
		Group:    hcGroup,
		Version:  hcVersion,
		Resource: hcResourcePlural,
	}

	hostclusterGVK = schema.GroupVersionKind{
		Group:   hcGroup,
		Version: hcVersion,
		Kind:    hcKind,
	}
)

const (
	defaultTimeout = 30 * time.Second

	defaultImagePullSecret = "upbound-pull-secret"
	ns                     = "upbound-system"

	jsonKey = "_json_key"

	errReadTokenFile          = "unable to read token file"
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

func init() {
	runtime.ErrorHandlers = []func(error){}
}

// BeforeApply sets default values in login before assignment and validation.
func (c *initCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *initCmd) AfterApply(insCtx *install.Context, kongCtx *kong.Context, quiet config.QuietFlag) error { //nolint:gocyclo
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	b, err := io.ReadAll(c.TokenFile)
	defer c.TokenFile.Close()
	if err != nil {
		return errors.Wrap(err, errReadTokenFile)
	}
	c.token = string(b)
	prereqs, err := prerequistes.New(insCtx.Kubeconfig)
	if err != nil {
		return err
	}
	c.prereqs = prereqs
	c.id = jsonKey
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
	mgr, err := helm.NewManager(insCtx.Kubeconfig,
		spacesChart,
		c.Repo,
		helm.WithNamespace(ns),
		helm.WithBasicAuth(c.id, c.token),
		helm.IsOCI(),
		helm.WithChart(c.Bundle),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.helmMgr = mgr

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

// initCmd installs Upbound.
type initCmd struct {
	helmMgr    install.Manager
	prereqs    *prerequistes.Manager
	parser     install.ParameterParser
	kClient    kubernetes.Interface
	dClient    dynamic.Interface
	prompter   input.Prompter
	pullSecret *kube.ImagePullApplicator
	id         string
	token      string
	quiet      config.QuietFlag

	Version string `arg:"" help:"Upbound Spaces version to install."`

	commonParams
	install.CommonParams

	Flags upbound.Flags `embed:""`
}

// Run executes the install command.
func (c *initCmd) Run(insCtx *install.Context, upCtx *upbound.Context) error {
	ctx := context.Background()

	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}

	// check if required prerequisites are installed
	status := c.prereqs.Check()

	// At least 1 prerequiste is not installed, check if we should install the
	// missing for the client.
	if len(status.NotInstalled) > 0 {
		pterm.Warning.Printfln("One or more required prerequistes are not installed.")
		pterm.DefaultInteractiveConfirm.DefaultText = "Would you like to install them now?"
		pterm.Println() // Blank line
		result, _ := pterm.DefaultInteractiveConfirm.Show()
		pterm.Println() // Blank line
		// pterm.Info.Printfln("You answered: %s", boolToText(result))

		if !result {
			pterm.Error.Println("prerequistes must be met inorder to proceed with installation")
			return nil
		}

		if err := c.installPrereqs(ctx); err != nil {
			return err
		}
	}

	pterm.Info.Printfln("Required prerequistes met!")
	pterm.Info.Printfln("Proceeding with Upbound Spaces installation...")

	if err := c.applySecret(ctx, ns); err != nil {
		return err
	}

	if err := c.deploySpace(context.Background(), params); err != nil {
		return err
	}

	pterm.Info.WithPrefix(upterm.RaisedPrefix).Println("Your Upbound Space is Ready!")

	outputNextSteps()
	return nil
}

func (c *initCmd) installPrereqs(ctx context.Context) error {

	status := c.prereqs.Check()
	for i, p := range status.NotInstalled {
		if err := upterm.WrapWithSuccessSpinner(
			upterm.StepCounter(
				fmt.Sprintf("Installing %s", p.GetName()),
				i+1,
				len(status.NotInstalled),
			),
			upterm.CheckmarkSuccessSpinner,
			p.Install,
		); err != nil {
			return err
		}
	}
	return nil
}

func (c *initCmd) applySecret(ctx context.Context, namespace string) error {
	creatPullSecret := func() error {
		if err := c.pullSecret.Apply(
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

	_, err := c.kClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, errCreateNamespace)
	}

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter(fmt.Sprintf("Creating pull secret %s", defaultImagePullSecret), 1, 3),
		upterm.CheckmarkSuccessSpinner,
		creatPullSecret,
	); err != nil {
		return err
	}
	return nil
}

func (c *initCmd) deploySpace(ctx context.Context, params map[string]any) error {
	install := func() error {
		if err := c.helmMgr.Install(strings.TrimPrefix(c.Version, "v"), params); err != nil {
			return err
		}
		return nil
	}

	if c.quiet {
		return install()
	}

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Initializing Space components", 2, 3),
		upterm.CheckmarkSuccessSpinner,
		install,
	); err != nil {
		return err
	}

	hcSpinner, _ := upterm.CheckmarkSuccessSpinner.Start(upterm.StepCounter("Starting Space Components", 3, 3))

	errC, err := kube.DynamicWatch(ctx, c.dClient.Resource(hostclusterGVR), &watcherTimeout, func(u *unstructured.Unstructured) (bool, error) {
		up := resources.HostCluster{Unstructured: *u}
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
	hcSpinner.Success()
	return nil
}

func outputNextSteps() {
	pterm.Println()
	pterm.Info.WithPrefix(upterm.EyesPrefix).Println("Next Steps 👇")
	pterm.Println()
	pterm.Println("👉 Check out spaces docs @ https://docs.upbound.io")
}
