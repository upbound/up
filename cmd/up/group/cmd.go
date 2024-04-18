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

package group

import (
	"context"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	spacesv1beta1 "github.com/upbound/up-sdk-go/apis/spaces/v1beta1"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/profile"
	"github.com/upbound/up/internal/upbound"
)

var (
	fieldNames = []string{"NAME", "PROTECTED"}
)

func init() {
	runtime.Must(spacesv1beta1.AddToScheme(scheme.Scheme))
}

// BeforeReset is the first hook to run.
func (c *Cmd) BeforeReset(p *kong.Path, maturity feature.Maturity) error {
	return feature.HideMaturity(p, maturity)
}

// AfterApply constructs and binds an Upbound context to any subcommands
// that have Run() methods that receive it.
func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	upCtx, err := upbound.NewFromFlags(c.Flags)
	if err != nil {
		return err
	}
	kongCtx.Bind(upCtx)

	if !upCtx.Profile.IsSpace() {
		// TODO: add legacy support
		return errors.New("Only Spaces contexts supported for now.")
	}
	return nil
}

// Cmd contains commands for interacting with groups.
type Cmd struct {
	Create createCmd `cmd:"" help:"Create a group."`
	Delete deleteCmd `cmd:"" help:"Delete a group."`
	List   listCmd   `cmd:"" help:"List groups in the space."`
	Get    getCmd    `cmd:"" help:"Get a group."`

	// Common Upbound API configuration
	Flags upbound.Flags `embed:""`
}

func (c *Cmd) Help() string {
	return `
Interact with groups within the current space. Both Upbound profiles and
local Spaces are supported. Use the "profile" management command to switch
between different Upbound profiles or to connect to a local Space.`
}

func extractGroupFields(obj any) []string {
	resp, ok := obj.(corev1.Namespace)
	if !ok {
		return []string{"unknown", "unknown"}
	}

	protected := false
	if av, ok := resp.ObjectMeta.Labels[spacesv1beta1.ControlPlaneGroupProtectionKey]; ok {
		if val, err := strconv.ParseBool(av); err == nil {
			protected = val
		}
	}

	return []string{
		resp.GetObjectMeta().GetName(),
		strconv.FormatBool(protected),
	}
}

func getCurrentProfile(ctx context.Context, upCtx *upbound.Context) (*profile.Profile, error) {
	// get context
	_, currentProfile, ctp, err := upCtx.Cfg.GetCurrentContext(ctx)
	if err != nil {
		return nil, err
	}
	if currentProfile == nil {
		return nil, errors.New(profile.NoSpacesContextMsg)
	}
	if ctp.Namespace == "" {
		return nil, errors.New(profile.NoGroupMsg)
	}
	if ctp.Name != "" {
		return nil, errors.New("Cannot list control planes from inside a control plane, use `up ctx ..` to switch to a group level.")
	}
	return currentProfile, nil
}
