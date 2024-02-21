package space

import (
	"context"
	"net/url"

	"github.com/alecthomas/kong"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
)

type detachCmd struct {
	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`
}

func (c *detachCmd) AfterApply(kongCtx *kong.Context) error {
	registryURL, err := url.Parse(agentRegistry)
	if err != nil {
		return err
	}

	if err := c.Kube.AfterApply(); err != nil {
		return err
	}

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
	kongCtx.Bind(kClient)

	with := []helm.InstallerModifierFn{
		helm.WithNamespace("upbound-connect"),
		helm.IsOCI(),
	}

	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		registryURL,
		with...,
	)
	if err != nil {
		return err
	}
	kongCtx.Bind(mgr)

	return nil
}

// Run executes the install command.
func (c *detachCmd) Run(ctx context.Context, kClient *kubernetes.Clientset, mgr *helm.Installer) error {
	if err := mgr.Uninstall(); err != nil {
		return err
	}

	return kClient.CoreV1().Namespaces().Delete(ctx, "upbound-connect", v1.DeleteOptions{})
}
