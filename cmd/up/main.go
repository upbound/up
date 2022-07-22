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
	"math/rand"
	"time"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/cmd/up/upbound"
	"github.com/upbound/up/cmd/up/uxp"
	"github.com/upbound/up/cmd/up/xpkg"
	"github.com/upbound/up/cmd/up/xpls"
	"github.com/upbound/up/internal/version"
)

var _ = kong.Must(&cli)

type versionFlag bool

// BeforeApply indicates that we want to execute the logic before running any
// commands.
func (v versionFlag) BeforeApply(ctx *kong.Context) error { // nolint:unparam
	fmt.Fprintln(ctx.Stdout, version.GetVersion())
	ctx.Exit(0)
	return nil
}

var cli struct {
	Version versionFlag `short:"v" name:"version" help:"Print version and exit."`

	License licenseCmd `cmd:"" help:"Print Up license information."`

	Login        loginCmd        `cmd:"" help:"Login to Upbound."`
	Logout       logoutCmd       `cmd:"" help:"Logout of Upbound."`
	ControlPlane controlPlaneCmd `cmd:"" name:"controlplane" aliases:"ctp" group:"controlplane" help:"Interact with control planes."`

	Upbound upbound.Cmd `cmd:"" help:"Interact with Upbound."`
	UXP     uxp.Cmd     `cmd:"" help:"Interact with UXP."`
	XPKG    xpkg.Cmd    `cmd:"" help:"Interact with UXP packages."`
	XPLS    xpls.Cmd    `cmd:"" help:"Start xpls language server."`
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	ctx := kong.Parse(&cli,
		kong.Name("up"),
		kong.Description("The Upbound CLI"),
		kong.UsageOnError())
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
