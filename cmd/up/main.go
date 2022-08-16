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

package main

import (
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"

	"github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/organization"
	"github.com/upbound/up/cmd/up/profile"
	"github.com/upbound/up/cmd/up/repository"
	"github.com/upbound/up/cmd/up/upbound"
	"github.com/upbound/up/cmd/up/uxp"
	"github.com/upbound/up/cmd/up/xpkg"
	"github.com/upbound/up/cmd/up/xpls"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/version"

	// TODO(epk): Remove this once we upgrade kubernetes deps to 1.25
	// TODO(epk): Specifically, get rid of the k8s.io/client-go/client/auth/azure
	// and k8s.io/client-go/client/auth/gcp packages.
	// Embed Kubernetes client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type versionFlag bool

// BeforeApply indicates that we want to execute the logic before running any
// commands.
func (v versionFlag) BeforeApply(ctx *kong.Context) error { // nolint:unparam
	fmt.Fprintln(ctx.Stdout, version.GetVersion())
	ctx.Exit(0)
	return nil
}

// AfterApply configures global settings before executing commands.
func (c *cli) AfterApply(ctx *kong.Context) error { //nolint:unparam
	if c.Quiet {
		ctx.Stdout, ctx.Stderr = io.Discard, io.Discard
	}
	ctx.BindTo(pterm.DefaultBasicText.WithWriter(ctx.Stdout), (*pterm.TextPrinter)(nil))
	// TODO(hasheddan): configure pretty print styling to match Upbound
	// branding.
	if !c.Pretty {
		// NOTE(hasheddan): enabling styling can make processing output with
		// other tooling difficult.
		pterm.DisableStyling()
	}
	ctx.Bind(c.Quiet)
	return nil
}

type cli struct {
	Version versionFlag      `short:"v" name:"version" help:"Print version and exit."`
	Quiet   config.QuietFlag `short:"q" name:"quiet" help:"Suppress all output."`
	Pretty  bool             `name:"pretty" help:"Pretty print output."`

	License licenseCmd `cmd:"" help:"Print Up license information."`

	Login        loginCmd         `cmd:"" help:"Login to Upbound."`
	Logout       logoutCmd        `cmd:"" help:"Logout of Upbound."`
	ControlPlane controlplane.Cmd `cmd:"" name:"controlplane" aliases:"ctp" help:"Interact with control planes."`
	Profile      profile.Cmd      `cmd:"" help:"Interact with Upbound Profiles"`
	Organization organization.Cmd `cmd:"" name:"organization" aliases:"org" help:"Interact with organizations."`
	Repository   repository.Cmd   `cmd:"" name:"repository" aliases:"repo" help:"Interact with repositories."`
	Upbound      upbound.Cmd      `cmd:"" help:"Interact with Upbound."`
	UXP          uxp.Cmd          `cmd:"" help:"Interact with UXP."`
	XPKG         xpkg.Cmd         `cmd:"" help:"Interact with UXP packages."`
	XPLS         xpls.Cmd         `cmd:"" help:"Start xpls language server."`
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	c := cli{}
	ctx := kong.Parse(&c,
		kong.Name("up"),
		kong.Description("The Upbound CLI"),
		kong.UsageOnError())
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
