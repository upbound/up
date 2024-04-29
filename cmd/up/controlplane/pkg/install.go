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

package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/upbound/up/internal/kube"
	"github.com/upbound/up/internal/resources"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/xpkg"
)

const errUnknownPkgType = "provided package type is unknown"

// Supported package kinds.
const (
	ConfigurationKind = "Configuration"
	ProviderKind      = "Provider"
)

var (
	providerGVR = schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "providers",
	}

	configurationGVR = schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "configurations",
	}
)

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *installCmd) AfterApply(kongCtx *kong.Context, upCtx *upbound.Context) error {
	switch kongCtx.Selected().Vars()["package_type"] {
	case ProviderKind:
		c.gvr = providerGVR
		c.kind = ProviderKind
	case ConfigurationKind:
		c.gvr = configurationGVR
		c.kind = ConfigurationKind
	default:
		return errors.New(errUnknownPkgType)
	}

	kubeconfig, err := kube.GetKubeConfig(c.Kubeconfig)
	if err != nil {
		return err
	}
	if upCtx.WrapTransport != nil {
		kubeconfig.Wrap(upCtx.WrapTransport)
	}

	client, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}
	c.r = client.Resource(c.gvr)
	return nil
}

// installCmd installs a package.
type installCmd struct {
	gvr  schema.GroupVersionResource
	kind string

	r dynamic.NamespaceableResourceInterface

	Package string `arg:"" help:"Reference to the ${package_type}."`

	// NOTE(hasheddan): kong automatically cleans paths tagged with existingfile.
	Kubeconfig         string        `hidden:"" type:"existingfile" help:"No longer used. Please use the KUBECONFIG environment variable instead."`
	Name               string        `help:"Name of ${package_type}."`
	PackagePullSecrets []string      `help:"List of secrets used to pull ${package_type}."`
	Wait               time.Duration `short:"w" help:"Wait duration for successful ${package_type} installation."`
}

// Run executes the install command.
func (c *installCmd) Run(ctx context.Context, p pterm.TextPrinter, upCtx *upbound.Context) error {
	ref, err := name.ParseReference(c.Package, name.WithDefaultRegistry(upCtx.RegistryEndpoint.Hostname()))
	if err != nil {
		return err
	}
	if c.Name == "" {
		c.Name = xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	}
	packagePullSecrets := make([]corev1.LocalObjectReference, len(c.PackagePullSecrets))
	for i, s := range c.PackagePullSecrets {
		packagePullSecrets[i] = corev1.LocalObjectReference{
			Name: s,
		}
	}
	if _, err := c.r.Create(ctx, &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "pkg.crossplane.io/v1",
		"kind":       c.kind,
		"metadata": map[string]interface{}{
			"name": c.Name,
		},
		"spec": map[string]interface{}{
			"package":            ref.Name(),
			"packagePullSecrets": packagePullSecrets,
		},
	}}, v1.CreateOptions{}); err != nil {
		return err
	}

	// Return early if wait duration is not provided.
	if c.Wait == 0 {
		p.Printfln("%s installed", c.Name)
		return nil
	}

	s, _ := upterm.CheckmarkSuccessSpinner.Start(fmt.Sprintf("%s installed. Waiting to become healthy...", c.Name))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	t := int64(c.Wait.Seconds())
	errC, err := kube.DynamicWatch(ctx, c.r, &t, func(u *unstructured.Unstructured) (bool, error) {
		pkg := resources.Package{Unstructured: *u}
		if pkg.GetInstalled() && pkg.GetHealthy() {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if err := <-errC; err != nil {
		return err
	}

	s.Success(fmt.Sprintf("%s installed and healthy", c.Name))
	return nil
}
