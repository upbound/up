# `xpls`, the Crossplane language server

`xpls` (pronounced "cross please") is the Crossplane [language server] developed
by the Upbound team. It provides IDE features to any [LSP]-compatible editor.

# Supported IDEs

Currently `xpls` is known to work with the following IDEs:
* VSCode (utlizing the [vscode-up] extension)
* Goland (utilizing the [LSP support] plugin)

# Getting started

## Install current version of `up-ls` as `up`
These steps will walk you through setting up `up` with the new features locally.

1. Clone `http:s//github.com/upbound/up-ls` locally.
2. cd into up-ls and run `make install`

At this point you should have the up-ls version of `up`. (If you have a homebrew-installed up version, you must uninstall it using `brew uninstall up` or configure your path to find the one in GOPATH first.)

Running `up --version` should give you something similar to `v0.6.0-rc.0.102.****`.

## Getting started with VSCode
These steps will walk you through installing the VSCode extension that will
start interacting with `xpls` locally.

1. Clone https://github.com/upbound/vscode-up locally.
2. Run `npm install -g vsce` to install the `vsce` tooling.
3. cd into vscode-up and run `make package`
    * This step will produce a packaged version of the extension at `out/up-0.0.1-dev.vsix`
4. Add the new extension to VSCode
    * `code --install-extension out/up-0.0.1-dev.vsix`
5. Open VSCode and your package project.

## Getting started with Goland
These steps will walk you through installing the LSP plugin that will start
interacting with `xpls` locally.

1. Open Goland
2. Preferences -> Plugins -> LSP Support -> Install
3. Reload Goland
4. Preferences -> Languages & Frameworks -> Language Server Protocol
    1. check both `log server communications` and `always send requests`
    2. Go to -> Server Definitions
        * Use the following settings:
            1. Executable
            2. Extension -> `yaml`
            3. Path -> location on disk where the up binary lives. For example `$GOPATH/bin/up` where $GOPATH here is fully resolved.
            4. Args -> `xpls serve --verbose`
5. Click `OK`
6. Within the IDE view, you should see an orb in the bottom bar indicating the connection status to `xpls`.
    

<!-- Named Links -->
[language server]: https://langserver.org/
[LSP]: https://microsoft.github.io/language-server-protocol/
[LSP support]: https://plugins.jetbrains.com/plugin/10209-lsp-support
[vscode-up]: https://github.com/upbound/vscode-up