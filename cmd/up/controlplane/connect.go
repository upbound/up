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
	"path"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfigurationscorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyconfigurationsmetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/yaml"

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
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	mgr, err := helm.NewManager(kubeconfig,
		connectorName,
		mcpRepoURL,
		helm.WithNamespace(c.Namespace))
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

// getCmd gets a single control plane in an account on Upbound.
type connectCmd struct {
	mgr     install.Manager
	parser  install.ParameterParser
	kClient kubernetes.Interface

	Name  string `arg:"" required:"" help:"Name of control plane."`
	Token string `required:"" help:"API token used to authenticate."`

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"MCP_CONNECTOR_NAMESPACE" default:"kube-system" help:"Kubernetes namespace for MCP Connector."`

	install.CommonParams
}

// Run executes the get command.
func (c *connectCmd) Run(p pterm.TextPrinter, upCtx *upbound.Context) error {
	if err := c.applyMCPKubeconfig(upCtx); err != nil {
		return err
	}
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	if err = c.mgr.Install("", params); err != nil {
		return err
	}

	if _, err = c.mgr.GetCurrentVersion(); err != nil {
		return err
	}

	p.Printfln("Connected to the MCP %q, ready for binding APIs!", c.Name)
	return nil
}

func (c *connectCmd) applyMCPKubeconfig(upCtx *upbound.Context) error {
	// Create a secret with MCP Kubeconfig
	proxy := upCtx.ProxyEndpoint
	id := path.Join(upCtx.Account, c.Name)
	key := fmt.Sprintf(kube.UpboundKubeconfigKeyFmt, strings.ReplaceAll(id, "/", "-"))
	proxy.Path = path.Join(proxy.Path, id, kube.UpboundK8sResource)

	mcpCfg := &v1.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: []v1.NamedCluster{
			{
				Name: key,
				Cluster: v1.Cluster{
					Server: proxy.String(),
				},
			},
		},
		AuthInfos: []v1.NamedAuthInfo{
			{
				Name: key,
				AuthInfo: v1.AuthInfo{
					Token: c.Token,
				},
			},
		},
		Contexts: []v1.NamedContext{
			{
				Name: key,
				Context: v1.Context{
					Cluster:  key,
					AuthInfo: key,
				},
			},
		},
		CurrentContext: key,
	}

	b, err := yaml.Marshal(mcpCfg)
	if err != nil {
		return errors.Wrap(err, "cannot marshal MCP kubeconfig")
	}

	secretName := "mcp-config"
	versionV1 := "v1"
	kindSecret := "Secret"
	_, err = c.kClient.CoreV1().Secrets(c.Namespace).Apply(context.Background(), &applyconfigurationscorev1.SecretApplyConfiguration{
		TypeMetaApplyConfiguration: applyconfigurationsmetav1.TypeMetaApplyConfiguration{
			Kind:       &kindSecret,
			APIVersion: &versionV1,
		},
		ObjectMetaApplyConfiguration: &applyconfigurationsmetav1.ObjectMetaApplyConfiguration{
			Name:      &secretName,
			Namespace: &c.Namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": b,
		},
	}, metav1.ApplyOptions{FieldManager: "up"})

	return errors.Wrap(err, "cannot apply MCP kubeconfig secret")
}

func urlMustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
