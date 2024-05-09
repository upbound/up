// Copyright 2024 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package version contains version cmd
package version

import (
	"context"
	"flag"
	"fmt"

	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/alecthomas/kong"
	"github.com/upbound/up/internal/upterm"
	"github.com/upbound/up/internal/version"
)

const (
	errKubeConfig         = "failed to get kubeconfig"
	errCreateK8sClientset = "failed to connect to cluster"

	errGetCrossplaneVersion = "unable to get crossplane version. Is your kubecontext pointed at a control plane?"
	errGetSpacesVersion     = "unable to get spaces version. Is your kubecontext pointed at a Space?"
)

const (
	versionUnknown  = "unknown"
	versionTemplate = `{{with .Client -}}
Client:
  Version:	{{.Version}}
{{- end}}

{{- if ne .Server nil}}{{with .Server}}
Server:
  Crossplane Version:	{{.CrossplaneVersion}}
  Spaces Controller Version:	{{.SpacesControllerVersion}}
{{- end}}{{- end}}`
)

type clientVersion struct {
	Version string `json:"version,omitempty"`
}

type serverVersion struct {
	CrossplaneVersion       string `json:"crossplaneVersion,omitempty" yaml:"crossplaneVersion,omitempty"`
	SpacesControllerVersion string `json:"spacesControllerVersion,omitempty" yaml:"spacesControllerVersion,omitempty"`
}

type versionInfo struct {
	Client clientVersion  `json:"client" yaml:"client"`
	Server *serverVersion `json:"server,omitempty" yaml:"server,omitempty"`
}

type Cmd struct {
	Client bool `env:"" help:"If true, shows client version only (no server required)." json:"client,omitempty"`
}

// BeforeApply sets default values and parses flags
func (c *Cmd) BeforeApply() error {
	flag.BoolVar(&c.Client, "client", false, "If true, shows client version only (no server required).")
	flag.Parse()
	return nil
}

func (c *Cmd) Help() string {
	return `
Options:
  --client=false:
  If true, shows client version only (no server required).

Usage:
  up version [flags] [options]
`
}

func (c *Cmd) BuildVersionInfo(ctx context.Context, kongCtx *kong.Context) (v versionInfo) {
	v.Client.Version = version.GetVersion()

	if c.Client {
		return v
	}

	config, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errKubeConfig)
		return v
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errCreateK8sClientset)
		return v
	}

	v.Server = &serverVersion{}
	v.Server.CrossplaneVersion, err = FetchCrossplaneVersion(ctx, *clientset)
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errGetCrossplaneVersion)
	}
	if v.Server.CrossplaneVersion == "" {
		v.Server.CrossplaneVersion = versionUnknown
	}

	v.Server.SpacesControllerVersion, err = FetchSpacesVersion(ctx, *clientset)
	if err != nil {
		fmt.Fprintln(kongCtx.Stderr, errGetSpacesVersion)
	}
	if v.Server.SpacesControllerVersion == "" {
		v.Server.SpacesControllerVersion = versionUnknown
	}

	return v
}

func (c *Cmd) Run(ctx context.Context, kongCtx *kong.Context, printer upterm.Printer) error {
	v := c.BuildVersionInfo(ctx, kongCtx)

	return printer.PrintTemplate(v, versionTemplate)
}
