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

package controlplane

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
	"github.com/upbound/up-sdk-go/service/robots"
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
func (c *connectCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	if c.ClusterName == "" {
		c.ClusterName = c.Namespace
	}
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	mgr, err := helm.NewManager(kubeconfig,
		connectorName,
		mcpRepoURL,
		helm.WithNamespace(c.InstallationNamespace))
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

// connectCmd connects the current cluster to a control plane in an account on
// Upbound.
type connectCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Name      string `arg:"" required:"" help:"Name of control plane."`
	Namespace string `arg:"" required:"" help:"Namespace in the control plane where the claims of the cluster will be stored."`

	Token                 string `help:"API token used to authenticate. If not provided, a new robot and a token will be created."`
	ClusterName           string `help:"Name of the cluster connecting to the control plane. If not provided, the namespace argument value will be used."`
	Kubeconfig            string `type:"existingfile" help:"Override the default kubeconfig path."`
	InstallationNamespace string `short:"n" env:"MCP_CONNECTOR_NAMESPACE" default:"kube-system" help:"Kubernetes namespace for MCP Connector. Default is kube-system."`

	install.CommonParams
}

// Run executes the connect command.
func (c *connectCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	token, err := c.getToken(p, upCtx)
	if err != nil {
		return errors.Wrap(err, "failed to get token")
	}
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	params["mcp"] = map[string]string{
		"account":   upCtx.Account,
		"name":      c.Name,
		"namespace": c.Namespace,
		"host":      fmt.Sprintf("%s://%s", upCtx.ProxyEndpoint.Scheme, upCtx.ProxyEndpoint.Host),
		"token":     token,
	}
	if err = c.mgr.Install("", params); err != nil {
		return err
	}

	if _, err = c.mgr.GetCurrentVersion(); err != nil {
		return err
	}

	p.Printfln("Connected to the control plane %q !", c.Name)
	return nil
}

func (c *connectCmd) getToken(p pterm.TextPrinter, upCtx *upbound.Context) (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}
	cfg, err := upCtx.BuildSDKConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to build SDK config")
	}
	a, err := accounts.NewClient(cfg).Get(context.Background(), upCtx.Account)
	if err != nil {
		return "", errors.Wrap(err, "failed to get account details")
	}
	var tokenOwner tokens.TokenOwner
	switch a.Account.Type {
	case accounts.AccountOrganization:
		r, err := robots.NewClient(cfg).Create(context.Background(), &robots.RobotCreateParameters{
			Attributes: robots.RobotAttributes{
				Name:        c.ClusterName,
				Description: "A robot used by the MCP Connector running in " + c.ClusterName,
			},
			Relationships: robots.RobotRelationships{
				Owner: robots.RobotOwner{
					Data: robots.RobotOwnerData{
						Type: "organization",
						ID:   upCtx.Account,
					},
				},
			},
		})
		if err != nil {
			return "", errors.Wrap(err, "failed to create robot")
		}
		p.Printfln("Created a robot account named %q.", c.ClusterName)
		tokenOwner = tokens.TokenOwner{
			Data: tokens.TokenOwnerData{
				Type: tokens.TokenOwnerRobot,
				ID:   r.ID.String(),
			},
		}
		p.Println("Creating a token for the robot account. This token will be" +
			"used to authenticate the cluster.")
	case accounts.AccountUser:
		tokenOwner = tokens.TokenOwner{
			Data: tokens.TokenOwnerData{
				Type: tokens.TokenOwnerUser,
				ID:   strconv.Itoa(int(a.User.ID)),
			},
		}
		p.Println("Creating a token for the user account. This token will be" +
			"used to authenticate the cluster.")
	default:
		return "", errors.New("only organization and user accounts are supported")
	}
	resp, err := tokens.NewClient(cfg).Create(context.Background(), &tokens.TokenCreateParameters{
		Attributes: tokens.TokenAttributes{
			Name: c.ClusterName,
		},
		Relationships: tokens.TokenRelationships{
			Owner: tokenOwner,
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to create token")
	}
	p.Printfln("Created a token named %q", c.ClusterName)
	return fmt.Sprint(resp.DataSet.Meta["jwt"]), nil
}

func urlMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
