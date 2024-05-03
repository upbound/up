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
	"context"
	"io"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	"github.com/pterm/pterm"
	"github.com/willabides/kongplete"

	"github.com/upbound/up/cmd/up/configuration"
	"github.com/upbound/up/cmd/up/configuration/template"
	"github.com/upbound/up/cmd/up/controlplane"
	"github.com/upbound/up/cmd/up/ctx"
	"github.com/upbound/up/cmd/up/group"
	"github.com/upbound/up/cmd/up/login"
	"github.com/upbound/up/cmd/up/migration"
	"github.com/upbound/up/cmd/up/organization"
	"github.com/upbound/up/cmd/up/profile"
	"github.com/upbound/up/cmd/up/query"
	"github.com/upbound/up/cmd/up/repository"
	"github.com/upbound/up/cmd/up/robot"
	"github.com/upbound/up/cmd/up/space"
	"github.com/upbound/up/cmd/up/trace"
	tviewtemplate "github.com/upbound/up/cmd/up/tview-template"
	"github.com/upbound/up/cmd/up/upbound"
	"github.com/upbound/up/cmd/up/uxp"
	v "github.com/upbound/up/cmd/up/version"
	"github.com/upbound/up/cmd/up/xpkg"
	"github.com/upbound/up/cmd/up/xpls"
	"github.com/upbound/up/internal/config"
	"github.com/upbound/up/internal/feature"
	"github.com/upbound/up/internal/upterm"

	// TODO(epk): Remove this once we upgrade kubernetes deps to 1.25
	// TODO(epk): Specifically, get rid of the k8s.io/client-go/client/auth/azure
	// and k8s.io/client-go/client/auth/gcp packages.
	// Embed Kubernetes client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

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

	printer := upterm.DefaultObjPrinter
	printer.Format = c.Format
	printer.Pretty = c.Pretty
	printer.Quiet = c.Quiet

	ctx.Bind(printer)
	ctx.Bind(c.Quiet)
	return nil
}

// BeforeReset runs before all other hooks. Default maturity level is stable.
func (c *cli) BeforeReset(ctx *kong.Context, p *kong.Path) error {
	ctx.Bind(feature.Stable)
	// If no command is selected, we are emitting help and filter maturity.
	if ctx.Selected() == nil {
		return feature.HideMaturity(p, feature.Stable)
	}
	return nil
}

type cli struct {
	Format config.Format    `name:"format" enum:"default,json,yaml" default:"default" help:"Format for get/list commands. Can be: json, yaml, default"`
	Quiet  config.QuietFlag `short:"q" name:"quiet" help:"Suppress all output."`
	Pretty bool             `name:"pretty" help:"Pretty print output."`

	License licenseCmd `cmd:"" help:"Print Up license information."`

	Help               helpCmd                      `cmd:"" help:"Show help."`
	Login              login.LoginCmd               `cmd:"" help:"Login to Upbound."`
	Logout             login.LogoutCmd              `cmd:"" help:"Logout of Upbound."`
	Configuration      configuration.Cmd            `cmd:"" name:"configuration" aliases:"cfg" help:"Interact with configurations."`
	ControlPlane       controlplane.Cmd             `cmd:"" hidden:"" name:"controlplane" aliases:"ctp" help:"Interact with control planes in the current context, both in the cloud and in a local space."`
	Space              space.Cmd                    `cmd:"" help:"Interact with Spaces."`
	Group              group.Cmd                    `cmd:"" help:"Interact with groups inside Spaces."`
	Organization       organization.Cmd             `cmd:"" name:"organization" aliases:"org" help:"Interact with Upbound organizations."`
	Profile            profile.Cmd                  `cmd:"" help:"Interact with Upbound profiles or local Spaces."`
	Ctx                ctx.Cmd                      `cmd:"" maturity:"alpha" hidden:"" help:"Select an Upbound kubeconfig context."`
	Repository         repository.Cmd               `cmd:"" name:"repository" aliases:"repo" help:"Interact with repositories."`
	Robot              robot.Cmd                    `cmd:"" name:"robot" help:"Interact with robots."`
	UXP                uxp.Cmd                      `cmd:"" help:"Interact with UXP."`
	XPKG               xpkg.Cmd                     `cmd:"" help:"Interact with UXP packages."`
	XPLS               xpls.Cmd                     `cmd:"" help:"Start xpls language server."`
	Alpha              alpha                        `cmd:"" help:"Alpha features. Commands may be removed in future releases."`
	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
	Version            v.Cmd                        `cmd:"" help:"Print the client and server version information for the current context."`
}

type helpCmd struct{}

func (h *helpCmd) Run(ctx *kong.Context) error {
	_, err := ctx.Parse([]string{"--help"})
	return err
}

// BeforeReset runs before all other hooks. If command has alpha as an ancestor,
// maturity level will be set to alpha.
func (a *alpha) BeforeReset(ctx *kong.Context) error { //nolint:unparam
	ctx.Bind(feature.Alpha)
	return nil
}

type alpha struct {
	// For now, we maintain compatibility for systems that may still use the alpha variant.
	// This nudges users towards the stable variant when they attempt to emit help.
	ControlPlane  controlplane.Cmd  `cmd:"" hidden:"" name:"controlplane" aliases:"ctp" help:"Interact with control planes in the current context, both in the cloud and in a local space."`
	Upbound       upbound.Cmd       `cmd:"" maturity:"alpha" help:"Interact with Upbound."`
	XPKG          xpkg.Cmd          `cmd:"" maturity:"alpha" help:"Interact with UXP packages."`
	Migration     migration.Cmd     `cmd:"" maturity:"alpha" help:"Migrate control planes to Upbound Managed Control Planes."`
	Trace         trace.Cmd         `cmd:"" maturity:"alpha" hidden:"" help:"Trace a Crossplane resource."`
	TviewTemplate tviewtemplate.Cmd `cmd:"" maturity:"alpha" hidden:"" help:"TView example."`
	Query         query.QueryCmd    `cmd:"" maturity:"alpha" hidden:"" help:"Query objects in one or many control planes."`
	Get           query.GetCmd      `cmd:"" maturity:"alpha" hidden:"" help:"Get objects in the current control plane."`
	WebLogin      login.LoginWebCmd `cmd:"" maturity:"alpha" help:"Use web browser to login to up cli."`
	Space         space.Cmd         `cmd:"" maturity:"alpha" help:"Interact with Spaces."`
	Ctx           ctx.Cmd           `cmd:"" maturity:"alpha" help:"Select an Upbound kubeconfig context."`
}

func main() {
	c := cli{}

	parser := kong.Must(&c,
		kong.Name("up"),
		kong.Description("The Upbound CLI"),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}))

	kongplete.Complete(parser,
		kongplete.WithPredictor("orgs", organization.PredictOrgs()),
		kongplete.WithPredictor("ctps", controlplane.PredictControlPlanes()),
		kongplete.WithPredictor("repos", repository.PredictRepos()),
		kongplete.WithPredictor("robots", robot.PredictRobots()),
		kongplete.WithPredictor("profiles", profile.PredictProfiles()),
		kongplete.WithPredictor("configs", configuration.PredictConfigurations()),
		kongplete.WithPredictor("templates", template.PredictTemplates()),
		// TODO(sttts): add get and query
	)

	if len(os.Args) == 1 {
		_, err := parser.Parse([]string{"--help"})
		parser.FatalIfErrorf(err)
		return
	}

	kongCtx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		defer cancel()
		<-sigCh
		kongCtx.Exit(1)
	}()

	kongCtx.BindTo(ctx, (*context.Context)(nil))
	kongCtx.FatalIfErrorf(kongCtx.Run())
}
