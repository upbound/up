// Copyright 2022 Upbound Inc
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

package connector

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/upbound/up-sdk-go/service/accounts"
	"github.com/upbound/up-sdk-go/service/tokens"

	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

var (
	mcpRepoURL = urlMustParse("https://charts.upbound.io/beta")
)

const (
	connectorName = "mcp-connector"

	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
)

// AfterApply sets default values in command after assignment and validation.
func (c *installCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	if c.ClusterName == "" {
		c.ClusterName = c.Namespace
	}
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}
	if upCtx.WrapTransport != nil {
		kubeconfig.Wrap(upCtx.WrapTransport)
	}

	mgr, err := helm.NewManager(kubeconfig,
		connectorName,
		mcpRepoURL,
		helm.WithNamespace(c.InstallationNamespace),
		helm.Wait(),
	)
	if err != nil {
		return err
	}
	c.mgr = mgr
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client

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

// installCmd connects the current cluster to a control plane in an account on
// Upbound.
type installCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Name      string `arg:"" required:"" help:"Name of control plane." predictor:"ctps"`
	Namespace string `arg:"" required:"" help:"Namespace in the control plane where the claims of the cluster will be stored."`

	Token                 string `help:"API token used to authenticate. If not provided, a new robot and a token will be created."`
	ClusterName           string `help:"Name of the cluster connecting to the control plane. If not provided, the namespace argument value will be used."`
	Kubeconfig            string `type:"existingfile" help:"Override the default kubeconfig path."`
	InstallationNamespace string `short:"n" env:"MCP_CONNECTOR_NAMESPACE" default:"kube-system" help:"Kubernetes namespace for MCP Connector. Default is kube-system."`
	ControlPlaneSecret    string `help:"Name of the secret that contains the kubeconfig for a control plane."`

	install.CommonParams
}

// Run executes the connect command.
func (c *installCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	token := "not defined"
	var err error

	if !upCtx.Profile.IsSpace() {
		token, err = c.getToken(p, upCtx)
		if err != nil {
			return errors.Wrap(err, "failed to get token")
		}
	}
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	// Some of these settings are only applicable if pointing to an Upbound
	// Cloud control plane. We leave them consistent since they won't impact
	// our ability to point the connector at Space control plane.
	params["mcp"] = map[string]string{
		"account":   upCtx.Account,
		"name":      c.Name,
		"namespace": c.Namespace,
		"host":      fmt.Sprintf("%s://%s", upCtx.ProxyEndpoint.Scheme, upCtx.ProxyEndpoint.Host),
		"token":     token,
	}

	// If the control-plane-secret has been specified, disable provisioning
	// the mcp-kubeconfig secret in favor of the supplied secret name.
	if c.ControlPlaneSecret != "" {
		v := params["mcp"]
		param := v.(map[string]any)
		param["secret"] = map[string]string{
			"name":      c.ControlPlaneSecret,
			"provision": "false",
		}

		params["mcp"] = param
	}

	p.Printfln("Installing %s to kube-system. This may take a few minutes.", connectorName)
	if err = c.mgr.Install("", params); err != nil {
		return err
	}

	if _, err = c.mgr.GetCurrentVersion(); err != nil {
		return err
	}

	p.Printfln("Connected to the control plane %s.", c.Name)
	p.Println("See available APIs with the following command: \n\n$ kubectl api-resources")
	return nil
}

func (c *installCmd) getToken(p pterm.TextPrinter, upCtx *upbound.Context) (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to build SDK config")
	}
	// NOTE(muvaf): We always use the querying user's account to create a token
	// assuming it has enough privileges. The ideal is to create a robot and a
	// token when the "--account" flag points to an organization but the robots
	// don't have default permissions, hence it'd require creation of team,
	// membership and also control plane permission to make it work.
	//
	// This is why this command is currently under alpha because we need to be
	// able to connect for organizations in a scalable way, i.e. every cluster
	// should have its own robot account.
	a, err := accounts.NewClient(cfg).Get(context.Background(), upCtx.Profile.ID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get account details")
	}
	p.Printfln("Creating an API token for the user %s. This token will be "+
		"used to authenticate the cluster.", a.User.Username)
	resp, err := tokens.NewClient(cfg).Create(context.Background(), &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.ClusterName,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokens.TokenOwner{
				Data: tokens.TokenOwnerData{
					Type: tokens.TokenOwnerUser,
					ID:   strconv.Itoa(int(a.User.ID)),
				},
			},
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to create token")
	}
	p.Printfln("Created a token named %s.", c.ClusterName)
	return fmt.Sprint(resp.DataSet.Meta["jwt"]), nil
}

func urlMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
