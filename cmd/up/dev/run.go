package dev

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/kind/pkg/apis/config/defaults"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/uxp"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
)

const (
	chartName             = "universal-crossplane"
	controlPlaneName      = "up-run"
	controlPlaneNamespace = "crossplane-system"
)

// runCmd runs a local control plane.
type runCmd struct{}

func (c *runCmd) Run(ctx context.Context) error {

	logger := cmd.NewLogger()

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
	)

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

	ver, err := c.installUXP(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to install UXP into the control plane")
	}

	fmt.Println(ver)

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
