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
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/alecthomas/kong"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pterm/pterm"
	"golang.org/x/exp/maps"
	"helm.sh/helm/v3/pkg/chart"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"

	upboundv1alpha1 "github.com/upbound/up-sdk-go/apis/upbound/v1alpha1"
	"github.com/upbound/up/cmd/up/space/defaults"
	spacefeature "github.com/upbound/up/cmd/up/space/features"
	"github.com/upbound/up/cmd/up/space/prerequisites"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/version"
)

const (
	hcGroup          = "internal.spaces.upbound.io"
	hcVersion        = "v1alpha1"
	hcResourcePlural = "xhostclusters"
)

var (
	watcherTimeout int64 = 600

	hostclusterGVR = schema.GroupVersionResource{
		Group:    hcGroup,
		Version:  hcVersion,
		Resource: hcResourcePlural,
	}

	defaultAcct = "disconnected"
)

const (
	defaultTimeout = 30 * time.Second

	defaultImagePullSecret = "upbound-pull-secret"
	ns                     = "upbound-system"

	errReadTokenFile          = "unable to read token file"
	errReadTokenFileData      = "unable to extract parameters from token file"
	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
	errCreateImagePullSecret  = "failed to create image pull secret"
	errFmtCreateNamespace     = "failed to create namespace %q"
	errCreateSpace            = "failed to create Space"
)

// initCmd installs Upbound Spaces.
type initCmd struct {
	Kube     upbound.KubeFlags       `embed:""`
	Registry authorizedRegistryFlags `embed:""`
	install.CommonParams
	Upbound upbound.Flags `embed:""`

	Version       string `arg:"" help:"Upbound Spaces version to install."`
	Yes           bool   `name:"yes" type:"bool" help:"Answer yes to all questions"`
	PublicIngress bool   `name:"public-ingress" type:"bool" help:"For AKS,EKS,GKE expose ingress publically"`

	helmMgr    install.Manager
	prereqs    *prerequisites.Manager
	helmParams map[string]any
	kClient    kubernetes.Interface
	dClient    dynamic.Interface
	pullSecret *kube.ImagePullApplicator
	quiet      config.QuietFlag
	features   *feature.Flags
}

func init() {
	// NOTE(tnthornton) we override the runtime.ErrorHandlers so that Helm
	// doesn't leak Println logs.
	kruntime.ErrorHandlers = []func(error){} //nolint:reassign

	kruntime.Must(upboundv1alpha1.AddToScheme(scheme.Scheme))
}

// BeforeApply sets default values in login before assignment and validation.
func (c *initCmd) BeforeApply() error {
	c.Set = make(map[string]string)
	return nil
}

// AfterApply sets default values in command after assignment and validation.
func (c *initCmd) AfterApply(kongCtx *kong.Context, quiet config.QuietFlag) error { //nolint:gocyclo
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
	kongCtx.Bind(upCtx)

	kubeconfig := c.Kube.GetConfig()

	kClient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = kClient

	// set the defaults
	cloud := c.Set[defaults.ClusterTypeStr]
	defs, err := defaults.GetConfig(c.kClient, cloud)
	if err != nil {
		return err
	}
	// todo(avalanche123): Remove these defaults once we can default to using
	// Upbound IAM, through connected spaces, to authenticate users in the
	// cluster
	defs.SpacesValues["authentication.hubIdentities"] = "true"
	defs.SpacesValues["authorization.hubRBAC"] = "true"
	// User supplied values always override the defaults
	maps.Copy(defs.SpacesValues, c.Set)
	c.Set = defs.SpacesValues
	if !c.PublicIngress {
		defs.PublicIngress = false
	} else {
		pterm.Info.Println("Public ingress will be exposed")
	}

	secret := kube.NewSecretApplicator(kClient)
	c.pullSecret = kube.NewImagePullApplicator(secret)
	dClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.dClient = dClient
	mgr, err := helm.NewManager(kubeconfig,
		spacesChart,
		c.Registry.Repository,
		helm.WithNamespace(ns),
		helm.WithBasicAuth(c.Registry.Username, c.Registry.Password),
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
	parser := helm.NewParser(base, c.Set)
	c.helmParams, err = parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	c.features = &feature.Flags{}
	spacefeature.EnableFeatures(c.features, c.helmParams)

	prereqs, err := prerequisites.New(kubeconfig, defs, c.features, c.Version)
	if err != nil {
		return err
	}
	c.prereqs = prereqs

	c.quiet = quiet
	return nil
}

// Run executes the install command.
func (c *initCmd) Run(ctx context.Context, upCtx *upbound.Context) error { //nolint:gocyclo
	overrideRegistry(c.Registry.Repository.String(), c.helmParams)
	ensureAccount(upCtx, c.helmParams)

	if c.helmParams["account"] == defaultAcct {
		pterm.Warning.Println("No account name was provided. Spaces initialized without an account name cannot be attached to the Upbound console! This cannot be changed later.")
		confirm := pterm.DefaultInteractiveConfirm
		confirm.DefaultText = fmt.Sprintf("Would you like to proceed with the default account name %q?", defaultAcct)
		result, _ := confirm.Show()
		if !result {
			pterm.Error.Println("Not proceeding without an account name; use --account or `up login` to create a profile.")
			return nil
		}
	}

	// check if required prerequisites are installed
	status, err := c.prereqs.Check()
	if err != nil {
		pterm.Error.Println("error checking prerequisites status")
		return err
	}

	// At least 1 prerequisite is not installed, check if we should install the
	// missing ones for the client.
	if len(status.NotInstalled) > 0 {
		pterm.Warning.Printfln("One or more required prerequisites are not installed:")
		pterm.Println()
		for _, p := range status.NotInstalled {
			pterm.Println(fmt.Sprintf("❌ %s", p.GetName()))
		}

		if !c.Yes {
			pterm.Println() // Blank line
			confirm := pterm.DefaultInteractiveConfirm
			confirm.DefaultText = "Would you like to install them now?"
			result, _ := confirm.Show()
			pterm.Println() // Blank line
			if !result {
				pterm.Error.Println("prerequisites must be met in order to proceed with installation")
				return nil
			}
		}
		if err := c.installPrereqs(status); err != nil {
			return err
		}
	}

	pterm.Info.Printfln("Required prerequisites met!")
	pterm.Info.Printfln("Proceeding with Upbound Spaces installation...")

	if err := c.applySecret(ctx, &c.Registry, ns); err != nil {
		return err
	}

	if err := c.deploySpace(ctx, c.helmParams); err != nil {
		return err
	}

	pterm.Info.WithPrefix(upterm.RaisedPrefix).Println("Your Upbound Space is Ready!")

	outputNextSteps()

	return nil
}

func (c *initCmd) installPrereqs(status *prerequisites.Status) error {
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
			fmt.Println()
			fmt.Println()
			return err
		}
	}
	return nil
}

func (c *initCmd) applySecret(ctx context.Context, regFlags *authorizedRegistryFlags, namespace string) error {
	creatPullSecret := func() error {
		if err := c.pullSecret.Apply(
			ctx,
			defaultImagePullSecret,
			namespace,
			regFlags.Username,
			regFlags.Password,
			regFlags.Endpoint.String(),
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
		return errors.Wrap(err, fmt.Sprintf(errFmtCreateNamespace, ns))
	}

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter(fmt.Sprintf("Creating pull secret %s", defaultImagePullSecret), 1, 3),
		upterm.CheckmarkSuccessSpinner,
		creatPullSecret,
	); err != nil {
		fmt.Println()
		fmt.Println()
		return err
	}
	return nil
}

func initVersionBounds(ch *chart.Chart) error {
	return checkVersion(fmt.Sprintf("unsupported chart version %q", ch.Metadata.Version), initVersionConstraints, ch.Metadata.Version)
}

func upVersionBounds(ch *chart.Chart) error {
	s, found := ch.Metadata.Annotations[chartAnnotationUpConstraints]
	if !found {
		return nil
	}
	constraints, err := parseChartUpConstraints(s)
	if err != nil {
		return fmt.Errorf("up version constraints %q provided by the chart are invalid: %w", s, err)
	}

	return checkVersion(fmt.Sprintf("unsupported up version %q", version.Version()), constraints, version.Version())
}

func (c *initCmd) deploySpace(ctx context.Context, params map[string]any) error {
	install := func() error {
		if err := c.helmMgr.Install(strings.TrimPrefix(c.Version, "v"), params, initVersionBounds, upVersionBounds); err != nil {
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
		fmt.Println()
		fmt.Println()
		return err
	}

	hcSpinner, _ := upterm.CheckmarkSuccessSpinner.Start(upterm.StepCounter("Starting Space Components", 3, 3))

	version, _ := semver.NewVersion(c.Version)
	requiresUXP, _ := semver.NewConstraint("< v1.7.0-0")

	if requiresUXP.Check(version) {
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
	}
	hcSpinner.Success()
	return nil
}

func outputNextSteps() {
	pterm.Println()
	pterm.Info.WithPrefix(upterm.EyesPrefix).Println("Next Steps 👇")
	pterm.Println()
	pterm.Println("👉 Check out Upbound Spaces docs @ https://docs.upbound.io/concepts/upbound-spaces")
}
