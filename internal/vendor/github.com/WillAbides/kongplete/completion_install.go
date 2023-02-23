package kongplete

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/riywo/loginshell"
)

// InstallCompletions is a kong command for installing or uninstalling shell completions
type InstallCompletions struct {
	Uninstall bool
}

// BeforeApply installs completion into the users shell.
func (c *InstallCompletions) BeforeApply(ctx *kong.Context) error {
	err := installCompletionFromContext(ctx)
	if err != nil {
		return err
	}
	ctx.Exit(0)
	return nil
}

var shellInstall = map[string]string{
	"bash": "complete -C ${bin} ${cmd}\n",
	"zsh": `autoload -U +X bashcompinit && bashcompinit
complete -C ${bin} ${cmd}
`,
	"fish": `function __complete_${cmd}
    set -lx COMP_LINE (commandline -cp)
    test -z (commandline -ct)
    and set COMP_LINE "$COMP_LINE "
    ${bin}
end
complete -f -c ${cmd} -a "(__complete_${cmd})"
`,
}

// installCompletionFromContext writes shell completion for the given command.
func installCompletionFromContext(ctx *kong.Context) error {
	shell, err := loginshell.Shell()
	if err != nil {
		return errors.Wrapf(err, "couldn't determine user's shell")
	}
	bin, err := os.Executable()
	if err != nil {
		return errors.Wrapf(err, "couldn't find absolute path to ourselves")
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		return errors.Wrapf(err, "couldn't find absolute path to ourselves")
	}
	w := ctx.Stdout
	cmd := ctx.Model.Name
	return installCompletion(w, shell, cmd, bin)
}

// installCompletion writes shell completion for a command.
func installCompletion(w io.Writer, shell, cmd, bin string) error {
	script, ok := shellInstall[filepath.Base(shell)]
	if !ok {
		return errors.Errorf("unsupported shell %s", shell)
	}
	vars := map[string]string{"cmd": cmd, "bin": bin}
	fragment := os.Expand(script, func(s string) string {
		v, ok := vars[s]
		if !ok {
			return "$" + s
		}
		return v
	})
	_, err := fmt.Fprint(w, fragment)
	return err
}
