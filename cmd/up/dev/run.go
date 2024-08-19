package dev

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/kind/pkg/apis/config/defaults"
	"sigs.k8s.io/kind/pkg/cluster"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/upbound/up/cmd/up/uxp"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upterm"
)

const (
	chartName             = "universal-crossplane"
	controlPlaneName      = "up-run"
	controlPlaneNamespace = "crossplane-system"
)

// runCmd runs a local control plane.
type runCmd struct{}

func (c *runCmd) Run(ctx context.Context) error {
	// Turn on colored output for pterm.
	pterm.EnableStyling()

	pterm.Println("Creating local control plane...")
	startk8s := func() error {
		// Including the logger parameter will result in kind logging output
		// to the end user.
		provider := cluster.NewProvider()

		if err := provider.Create(
			controlPlaneName,
			cluster.CreateWithNodeImage(defaults.Image),
			// Removes the following block:
			/*
				Set kubectl context to "kind-up-run"
				You can now use your cluster with:

				kubectl cluster-info --context kind-up-run
			*/
			cluster.CreateWithDisplayUsage(false),
			// Removes 'Thanks for using kind! 😊'
			cluster.CreateWithDisplaySalutation(false),
		); err != nil {
			return errors.Wrap(err, "failed to create cluster")
		}
		return nil
	}

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Starting control plane", 1, 2),
		upterm.CheckmarkSuccessSpinner,
		startk8s,
	); err != nil {
		return errors.Wrap(err, "failed to print status")
	}

	startxp := func() error {
		_, err := c.installUXP(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to install UXP into the control plane")
		}
		return nil
	}

	if err := upterm.WrapWithSuccessSpinner(
		upterm.StepCounter("Starting Crossplane", 2, 2),
		upterm.CheckmarkSuccessSpinner,
		startxp,
	); err != nil {
		return errors.Wrap(err, "failed to print status")
	}

	outputNextSteps()
	return nil
}

// installUXP installs the UXP helm chart into the crossplane-system namespace.
// Currently we don't support customization, so any logic around supplying
// parameters is not included.
func (c *runCmd) installUXP(ctx context.Context) (string, error) {
	kubeconfig, err := kube.GetKubeConfig("")
	if err != nil {
		return "", errors.Wrap(err, "failed to get kubeconfig")
	}

	repo := uxp.RepoURL
	mgr, err := helm.NewManager(kubeconfig,
		chartName,
		repo,
		helm.WithNamespace(controlPlaneNamespace),
		helm.Wait(),
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to build new Helm manager")
	}

	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to build kubernetes client")
	}

	// Create namespace if it does not exist.
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: controlPlaneNamespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return "", errors.Wrapf(err, "failed to create %q namespace", controlPlaneNamespace)
	}

	// Install UXP Helm chart.
	if err = mgr.Install("", map[string]any{}); err != nil {
		return "", errors.Wrap(err, "failed to install UXP Helm chart")
	}

	// Get current version of UXP helm chart.
	curVer, err := mgr.GetCurrentVersion()
	if err != nil {
		return "", errors.Wrap(err, "failed to get current Helm chart version")
	}

	return curVer, nil
}

// outputNextSteps is a simple function that is intended to be used after the
// install operation.
func outputNextSteps() {
	pterm.Println()
	pterm.Info.WithPrefix(upterm.RaisedPrefix).Println("Your local control plane is ready 👀")
	pterm.Println()
}
