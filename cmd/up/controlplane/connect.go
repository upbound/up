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
	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/install/helm"
	"github.com/upbound/up/internal/kube"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"net/url"
	"os"
	"path"
	"sigs.k8s.io/yaml"
	"strings"

	"github.com/upbound/up/internal/upbound"

	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

var (
	repoURL, _ = url.Parse("https://charts.upbound.io/stable")
)

const (
	errReadParametersFile     = "unable to read parameters file"
	errParseInstallParameters = "unable to parse install parameters"
)

// AfterApply sets default values in command after assignment and validation.
func (c *connectCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kubeconfig, err := kube.GetKubeConfig("/Users/hasanturken/.kube/config")
	if err != nil {
		return err
	}

	mgr, err := helm.NewManager(kubeconfig,
		"mcp-connector",
		repoURL,
		helm.WithNamespace("kube-system"),
		helm.WithChart(os.NewFile(0, "/Users/hasanturken/Workspace/upbound/mcp-connector/_output/charts/mcp-connector-0.0.1.tgz")))
	if err != nil {
		return err
	}
	c.mgr = mgr
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.kClient = client
	c.kAggregator, err = aggregatorclient.NewForConfig(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "cannot create aggregator client")
	}
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
	mgr         install.Manager
	parser      install.ParameterParser
	kAggregator aggregatorclient.Interface
	kClient     kubernetes.Interface

	Name  string `arg:"" required:"" help:"Name of control plane."`
	For   string `required:"" help:"APIVersion of the resources to connect for."`
	Token string `required:"" help:"API token used to authenticate."`

	install.CommonParams
}

// Run executes the get command.
func (c *connectCmd) Run(p pterm.TextPrinter, cc *cp.Client, upCtx *upbound.Context) error {
	params, err := c.parser.Parse()
	if err != nil {
		return errors.Wrap(err, errParseInstallParameters)
	}
	if err = c.mgr.Install("0.0.1", params); err != nil {
		return err
	}

	_, err = c.mgr.GetCurrentVersion()
	if err != nil {
		return err
	}
	//p.Printfln("Installed %s version %s", "mcp-connector", curVer)

	// Create a secret with MCP Kubeconfig
	proxy := upCtx.ProxyEndpoint
	id := path.Join(upCtx.Account, c.Name)
	key := fmt.Sprintf("upbound-%s", strings.ReplaceAll(id, "/", "-"))
	proxy.Path = path.Join(proxy.Path, id, "k8s")

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
		return errors.Wrap(err, "cannot marshal kubeconfig")
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcp-config",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"kubeconfig": b,
		},
	}
	_, err = c.kClient.CoreV1().Secrets("kube-system").Create(context.Background(), &secret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot create mcp kubeconfig secret")
	}

	// Deploy APIService for the requested Group/Version
	apiVersion := strings.Split(c.For, "/")
	_, err = c.kAggregator.ApiregistrationV1().APIServices().Create(context.Background(), &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiVersion[1] + "." + apiVersion[0],
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:   apiVersion[0],
			Version: apiVersion[1],
			Service: &apiregistrationv1.ServiceReference{
				Namespace: "kube-system",
				Name:      "mcp-connector",
			},
			GroupPriorityMinimum:  1000,
			VersionPriority:       15,
			InsecureSkipTLSVerify: true,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot create APIService")
	}
	p.Printfln("Connected to the MCP %s for %s!", c.Name, c.For)

	return nil
}
