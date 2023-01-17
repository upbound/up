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
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pterm/pterm"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	cp "github.com/upbound/up-sdk-go/service/controlplanes"
	"github.com/upbound/up/internal/install"
	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/upbound"
)

// AfterApply sets default values in command after assignment and validation.
func (c *bindCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}

	c.kDynamic, err = dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return errors.Wrap(err, "cannot create a dynamic client")
	}

	return nil
}

// bindCmd gets a single control plane in an account on Upbound.
type bindCmd struct {
	kDynamic dynamic.Interface

	APIVersion string `arg:"" required:"" help:"APIVersion of the resources to connect for. Expected as [group]/[version]"`

	Kubeconfig string `type:"existingfile" help:"Override default kubeconfig path."`
	Namespace  string `short:"n" env:"MCP_CONNECTOR_NAMESPACE" default:"kube-system" help:"Kubernetes namespace for MCP Connector."`

	install.CommonParams
}

// Run executes the get command.
func (c *bindCmd) Run(p pterm.TextPrinter, cc *cp.Client, upCtx *upbound.Context) error {
	// Deploy APIService for the requested Group/Version
	apiVersion := strings.Split(c.APIVersion, "/")
	if len(apiVersion) != 2 {
		return errors.New("invalid APIVersion format, expected as [group]/[version]")
	}
	_, err := c.kDynamic.Resource(schema.GroupVersionResource{
		Group:    "apiregistration.k8s.io",
		Version:  "v1",
		Resource: "apiservices",
	}).Create(context.Background(), &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiregistration.k8s.io/v1",
			"kind":       "APIService",
			"metadata": map[string]interface{}{
				"name": apiVersion[1] + "." + apiVersion[0],
			},
			"spec": map[string]interface{}{
				"group":   apiVersion[0],
				"version": apiVersion[1],
				"service": map[string]interface{}{
					"namespace": c.Namespace,
					"name":      connectorName,
				},
				"groupPriorityMinimum": 1000,
				"versionPriority":      15,
				// TODO(turkenh): remove this when MCP connector has proper TLS setup
				"insecureSkipTLSVerify": true,
			},
		},
	}, metav1.CreateOptions{})
	if resource.Ignore(kerrors.IsAlreadyExists, err) != nil {
		return errors.Wrap(err, "cannot create APIService")
	}

	p.Printfln("APIs under %s were bound to the Managed Control Plane!", c.APIVersion)

	return nil
}
