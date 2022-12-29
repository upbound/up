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
	"strings"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *bindCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kubeconfig, err := kube.GetKubeConfig("/Users/hasanturken/.kube/config")
	if err != nil {
		return err
	}

	c.kAggregator, err = aggregatorclient.NewForConfig(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "cannot create aggregator client")
	}
	return nil
}

// bindCmd gets a single control plane in an account on Upbound.
type bindCmd struct {
	kAggregator aggregatorclient.Interface

	APIVersion string `arg:"" required:"" help:"APIVersion of the resources to connect for."`
	Namespace  string `short:"n" env:"MCP_CONNECTOR_NAMESPACE" default:"kube-system" help:"Kubernetes namespace for MCP Connector."`

	install.CommonParams
}

// Run executes the get command.
func (c *bindCmd) Run(p pterm.TextPrinter, cc *cp.Client, upCtx *upbound.Context) error {
	// Deploy APIService for the requested Group/Version
	apiVersion := strings.Split(c.APIVersion, "/")
	_, err := c.kAggregator.ApiregistrationV1().APIServices().Create(context.Background(), &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: apiVersion[1] + "." + apiVersion[0],
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:   apiVersion[0],
			Version: apiVersion[1],
			Service: &apiregistrationv1.ServiceReference{
				Namespace: c.Namespace,
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
	p.Printfln("APIs under %s were bound to the Managed Control Plane!", c.APIVersion)

	return nil
}
