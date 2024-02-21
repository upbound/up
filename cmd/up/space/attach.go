package space

import (
	"context"
	"net/url"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/upbound/up/cmd/up/space/prerequisites"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

const (
	agentChart = "agent"

	// TODO(tnthornton) these can probably be replaced by our public chart
	// museum. This would allow us to use wildcards like mxp-connector.
	supportedVersion = "0.0.0-100.g216e157"
	agentRegistry    = "us-west1-docker.pkg.dev/orchestration-build/connect"

	// TODO(tnthornton) maybe move this to the agent chart?
	devConnectURL = "nats://connect.u5d.dev"
)

type attachCmd struct {
	Upbound upbound.Flags     `embed:""`
	Kube    upbound.KubeFlags `embed:""`

	helmMgr install.Manager
	prereqs *prerequisites.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface
	dClient dynamic.Interface
	quiet   config.QuietFlag
}

func (c *attachCmd) AfterApply(kongCtx *kong.Context) error {
	registryURL, err := url.Parse(agentRegistry)
	if err != nil {
		return err
	}

	if err := c.Kube.AfterApply(); err != nil {
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

	dClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.dClient = dClient
	mgr, err := helm.NewManager(kubeconfig,
		agentChart,
		registryURL,
		helm.WithNamespace("upbound-connect"),
		helm.CreateNamespace(true),
		helm.IsOCI(),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.helmMgr = mgr

	return nil
}

// Run executes the install command.
func (c *attachCmd) Run(ctx context.Context, upCtx *upbound.Context) error {
	if err := c.helmMgr.Install(supportedVersion, map[string]any{
		"nats": map[string]any{
			"url": devConnectURL,
		},
	}); err != nil {
		return err
	}

	return nil
}