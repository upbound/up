package main

import (
	"fmt"

	"github.com/alecthomas/kong"

	"github.com/upbound/up/internal/version"
)

var _ = kong.Must(&cli)

type versionFlag bool

// BeforeApply indicates that we want to execute the logic before running any
// commands.
func (v versionFlag) BeforeApply(ctx *kong.Context) error {
	fmt.Fprintln(ctx.Stdout, version.GetVersion())
	ctx.Exit(0)
	return nil
}

var cli struct {
	Version versionFlag `short:"v" name:"version" help:"Print version and exit."`
}

func main() {
	ctx := kong.Parse(&cli,
		kong.Name("up"),
		kong.Description("The Upbound CLI"),
		kong.UsageOnError())
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
