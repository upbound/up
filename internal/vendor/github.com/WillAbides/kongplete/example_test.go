// This example is adapted from the shell example in github.com/alecthomas/kong

package kongplete_test

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/willabides/kongplete"
)

var shellCli struct {
	Rm struct {
		User      string `help:"Run as user." short:"u" default:"default"`
		Force     bool   `help:"Force removal." short:"f"`
		Recursive bool   `help:"Recursively remove files." short:"r"`
		Hidden    string `help:"A hidden flag" hidden:""`

		Paths []string `arg:"" help:"Paths to remove." type:"path" name:"path" predictor:"file"`
	} `cmd:"" help:"Remove files."`

	Ls struct {
		Paths []string `arg:"" optional:"" help:"Paths to list." type:"path" predictor:"file"`
	} `cmd:"" help:"List paths."`

	Hidden struct{} `cmd:"" help:"A hidden command" hidden:""`

	Debug bool `help:"Debug mode."`

	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"install shell completions"`
}

func Example() {
	// Create a kong parser as usual, but don't run Parse quite yet.
	parser := kong.Must(&shellCli,
		kong.Name("shell"),
		kong.Description("A shell-like example app."),
		kong.UsageOnError(),
	)

	// Run kongplete.Complete to handle completion requests
	kongplete.Complete(parser,
		kongplete.WithPredictor("file", complete.PredictFiles("*")),
	)

	// Proceed as normal after kongplete.Complete.
	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	switch ctx.Command() {
	case "rm <path>":
		fmt.Println(shellCli.Rm.Paths, shellCli.Rm.Force, shellCli.Rm.Recursive)

	case "ls", "hidden":
	}
}
