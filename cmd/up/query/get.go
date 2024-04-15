// Copyright 2024 Upbound Inc
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

// Please note: As of March 2023, the `upbound` commands have been disabled.
// We're keeping the code here for now, so they're easily resurrected.
// The upbound commands were meant to support the Upbound self-hosted option.

package query

import (
	"fmt"

	"github.com/alecthomas/kong"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/query/resource"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

type GetCmd struct {
	cmd

	Namespace     string `short:"n" name:"namespace" help:"If present, the namespace scope for this CLI request."`
	AllNamespaces bool   `short:"A" name:"all-namespaces" help:"If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace."`
}

func (c *GetCmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *GetCmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	// load current kubeconfig context
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	ctpKubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	ctpConfig, err := ctpKubeconfig.ClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get rest config from kubeconfig")
	}

	// extract control plane from controlplane kubeconfig context
	// example: https://host/apis/spaces.upbound.io/v1beta1/namespaces/default/controlplanes/ctp-kine/k8s
	controlPlane, found := profile.ParseSpacesK8sURL(ctpConfig.Host)
	if !found {
		return errors.New("You are not connected to a control plane.")
	}

	// create Spaces API kubeconfig
	// TODO(sttts): here we have to continue with baseURL := m[1] to talk to Spaces API. For now we use the spaces profile instead.
	spacesKubeconfig, _, err := upCtx.Profile.GetSpaceRestConfig()
	if err != nil {
		return err
	}
	kongCtx.Bind(spacesKubeconfig)

	// default namespace flag from kubeconfig context
	if !c.AllNamespaces {
		c.namespace = c.Namespace // default to the flag
		if c.namespace == "" {
			c.namespace, _, err = ctpKubeconfig.Namespace()
			if err != nil {
				return errors.Wrap(err, "failed to get current namespace")
			}
		}
		if c.namespace == "" {
			c.namespace = "default"
		}
	}

	// create query template. The scope is always the current control plane.
	query := &resource.Query{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: controlPlane.Namespace,
			Name:      controlPlane.Name,
		},
	}
	kongCtx.BindTo(query, (*resource.QueryObject)(nil))

	// what to print if there is no resource found
	kongCtx.BindTo(NotFoundFunc(func() error {
		var err error
		if c.namespace != "" {
			_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in %q namespace in control plane %s/%s.\n", c.namespace, controlPlane.Namespace, controlPlane.Name)
		} else {
			_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in control plane %s/%s.\n", controlPlane.Namespace, controlPlane.Name)
		}
		return err
	}), (*NotFound)(nil))

	return c.afterApply()
}

func (c *GetCmd) Help() string {
	s, err := help("up alpha get") // nolint:errcheck // nothing we can do here.
	if err != nil {
		return err.Error()
	}
	return s
}
