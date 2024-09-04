// Copyright 2024 Upbound Inc
// Copyright 2014-2024 The Kubernetes Authors.
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

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/up/cmd/up/query/resource"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
)

type QueryCmd struct {
	cmd

	// flags about the scope
	Namespace    string `short:"n" name:"namespace" env:"UPBOUND_NAMESPACE" help:"Namespace name for resources to query. By default, it's all namespaces if not on a control plane profile   the profiles current namespace or \"default\"."`
	Group        string `short:"g" name:"group" env:"UPBOUND_GROUP" help:"Control plane group. By default, it's the kubeconfig's current namespace or \"default\"."`
	ControlPlane string `short:"c" name:"controlplane" env:"UPBOUND_CONTROLPLANE" help:"Control plane name. Defaults to the current kubeconfig context if it points to a control plane."`
	AllGroups    bool   `short:"A" name:"all-groups" help:"Query in all groups."`
}

// BeforeReset is the first hook to run.
func (c *QueryCmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *QueryCmd) AfterApply(kongCtx *kong.Context) error { // nolint:gocyclo // pure plumbing. Doesn't get better by splitting.
	upCtx, err := upbound.NewFromFlags(c.Flags, upbound.AllowMissingProfile())
	if err != nil {
		return err
	}
	upCtx.SetupLogging()

	kongCtx.Bind(upCtx)

	kubeconfig, err := upCtx.Kubecfg.ClientConfig()
	if err != nil {
		return err
	}

	// where are we?
	base, ctp, isSpace := upCtx.GetCurrentSpaceContextScope()
	if !isSpace {
		return errors.New("not connected to a Space. Use 'up ctx' to switch to a Space or control plane context.")
	}
	if ctp.Namespace != "" && ctp.Name != "" {
		// on a controlplane
		if c.AllGroups {
			return errors.Errorf("cannot use --all-groups in a control plane context. Use `up ctx ..' to switch to the Space.")
		}
		if c.Group != "" {
			return errors.Errorf("cannot use --group in a control plane context. Use `up ctx ..' to switch to the Space.")
		}
		c.Group = ctp.Namespace
		c.ControlPlane = ctp.Name

		// move from ctp URL to Spaces API in order to send Query API requests
		kubeconfig.Host = base
		kubeconfig.APIPath = ""
	} else if c.Group == "" && !c.AllGroups {
		// on the Spaces API
		if ctp.Namespace != "" {
			c.Group = ctp.Namespace
		}
	}

	kongCtx.Bind(kubeconfig)

	// create query template, kind depending on the scope
	var query resource.QueryObject
	switch {
	case c.Group != "" && c.ControlPlane != "":
		query = &resource.Query{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
				Name:      c.ControlPlane,
			},
		}
	case c.Group != "":
		query = &resource.GroupQuery{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: c.Group,
			},
		}
	default:
		query = &resource.SpaceQuery{}
	}
	kongCtx.BindTo(query, (*resource.QueryObject)(nil))

	// namespace in the control plane, logic is easy here
	c.namespace = c.Namespace

	// what to print if there is no resource found
	kongCtx.BindTo(NotFoundFunc(func() error {
		if c.namespace != "" {
			switch {
			case c.Group == "":
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in %q namespace in any control plane.\n", c.namespace)
			case c.ControlPlane == "":
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in %s namespace in control plane group %q.\n", c.namespace, c.Group)
			default:
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in %q namespace in control plane %s/%s.\n", c.namespace, c.Group, c.ControlPlane)
			}
		} else {
			switch {
			case c.Group == "":
				_, err = fmt.Fprintln(kongCtx.Stderr, "No resources found in any control plane.")
			case c.ControlPlane == "":
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in control plane group %q.\n", c.Group)
			default:
				_, err = fmt.Fprintf(kongCtx.Stderr, "No resources found in control plane %s/%s.\n", c.Group, c.ControlPlane)
			}
		}
		return err
	}), (*NotFound)(nil))

	return c.afterApply()
}

func (c *QueryCmd) Help() string {
	s, err := help("up alpha query") // nolint:errcheck // nothing we can do here.
	if err != nil {
		return err.Error()
	}
	return s
}
