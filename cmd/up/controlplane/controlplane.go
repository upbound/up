// Copyright 2021 Upbound Inc
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
	"time"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpcommonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"

	"github.com/upbound/up/cmd/up/controlplane/connector"
	"github.com/upbound/up/cmd/up/controlplane/pkg"
	"github.com/upbound/up/cmd/up/controlplane/pullsecret"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upbound"
	"github.com/upbound/up/internal/upterm"
)

var (
	spacefieldNames = []string{"GROUP", "NAME", "CROSSPLANE", "SYNCED", "READY", "MESSAGE", "AGE"}
)

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds a control plane client to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	// we can't use control planes from inside a control plane
	if _, ctp, isSpace := upCtx.GetCurrentSpaceContextScope(); isSpace && ctp.Name != "" {
		return errors.New("cannot view control planes from inside a control plane context. Use 'up ctx ..' to go up to the group context")
	}

	cl, err := upCtx.BuildCurrentContextClient()
	if err != nil {
		return errors.Wrap(err, "unable to get kube client")
	}
	kongCtx.BindTo(cl, (*client.Client)(nil))

	return nil
}

func PredictControlPlanes() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) (prediction []string) {
		upCtx, err := upbound.NewFromFlags(upbound.Flags{})
		if err != nil {
			return nil
		}

		client, err := upCtx.BuildCurrentContextClient()
		if err != nil {
			return nil
		}

		var l spacesv1beta1.ControlPlaneList
		if err := client.List(context.Background(), &l); err != nil {
			return nil
		}

		if len(l.Items) == 0 {
			return nil
		}

		data := make([]string, len(l.Items))
		for i, ctp := range l.Items {
			data[i] = ctp.Name
		}
		return data
	})
}

// Cmd contains commands for interacting with control planes.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a managed control plane."`
	Delete deleteCmd `cmd:"" help:"Delete a control plane."`
	List   listCmd   `cmd:"" help:"List control planes for the account."`
	Get    getCmd    `cmd:"" help:"Get a single control plane."`

	Connector connector.Cmd `cmd:"" help:"Connect an App Cluster to a managed control plane."`

	Configuration pkg.Cmd `cmd:"" set:"package_type=Configuration" help:"Manage Configurations."`
	Provider      pkg.Cmd `cmd:"" set:"package_type=Provider" help:"Manage Providers."`

	PullSecret pullsecret.Cmd `cmd:"" help:"Manage package pull secrets."`

	// Deprecated commands
	Connect    connectCmd    `cmd:"" help:"Deprecated: Connect kubectl to control plane."`
	Disconnect disconnectCmd `cmd:"" help:"Deprecated: Disconnect kubectl from control plane."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

func (c *Cmd) Help() string {
	return `
Interact with control planes of the current profile. Both Upbound profiles and
local Spaces are supported. Use the "profile" management command to switch
between different Upbound profiles or to connect to a local Space.`
}

func extractSpaceFields(obj any) []string {
	ctp, ok := obj.(spacesv1beta1.ControlPlane)
	if !ok {
		return []string{"unknown", "unknown", "", "", "", "", ""}
	}

	v := ""
	if pv := ctp.Spec.Crossplane.Version; pv != nil {
		v = *pv
	}

	return []string{
		ctp.GetNamespace(),
		ctp.GetName(),
		v,
		string(ctp.GetCondition(xpcommonv1.TypeSynced).Status),
		string(ctp.GetCondition(xpcommonv1.TypeReady).Status),
		ctp.Annotations["internal.spaces.upbound.io/message"],
		formatAge(ptr.To(time.Since(ctp.CreationTimestamp.Time))),
	}
}

func formatAge(age *time.Duration) string {
	if age == nil {
		return ""
	}

	return duration.HumanDuration(*age)
}

func tabularPrint(obj any, printer upterm.ObjectPrinter, upCtx *upbound.Context) error {
	return printer.Print(obj, spacefieldNames, extractSpaceFields)
}
