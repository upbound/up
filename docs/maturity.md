# API Maturity

`up` aims to be explicit about the level of support for individual commands. As
such, every command is denoted with an associated level of maturity. Currently,
there are only two levels:

- `alpha`: command is experimental and is likely to be removed in a future minor
  version release.
- `stable`: command is expected to be supported in perpetuity, but may be
  deprecated or removed in subsequent minor version releases.

The `stable` guarantee is expected to move to a backwards compatibility
guarantee once the `up` CLI reaches `v1.0`. 

The second command node in the graph (i.e. the noun after `up`) indicates the
maturity level of a given command. For example, all `alpha` commands will start
with `up alpha` (e.g. `up alpha xpkg xp-extract`). If the second noun is not a
maturity level, the command is `stable` (e.g. `up xpkg build`). This structure
is implemented to ensure that users are opting in to using commands that are
likely to change from one release to another.

## Hidden Commands

In special circumstances, `up` may attach a command node at multiple locations
in the graph, with only one variant being visible. For example, in the `v0.13.0`
release, which introduced the API maturity framework, some commands were moved
to `alpha` that were already widely depended upon by existing user workflows and
CI systems. In order to ensure that those commands did not immediately break on
upgrade, they remained as hidden at their previous location in the command
graph.

For example, `up ctp create` was moved to `up alpha ctp create` in `v0.13.0`.
Executing `up ctp create -h` will inform the user that the command exists, but
they are encouraged to use the unhidden variant. If a user does execute the
hidden command with valid arguments, it will run successfully.

```
$ up ctp create -h
Refusing to emit help for hidden command. See alpha variant.

$ up ctp create cool-plane
dan/cool-plane created

$ up alpha ctp create -h
Usage: up alpha controlplane (ctp) create <name>

Create a hosted control plane.

Arguments:
  <name>    Name of control plane.

Flags:
  -h, --help                         Show context-sensitive help.
  -v, --version                      Print version and exit.
  -q, --quiet                        Suppress all output.
      --pretty                       Pretty print output.

      --domain=https://upbound.io    Root Upbound domain ($UP_DOMAIN).
      --profile=STRING               Profile used to execute command ($UP_PROFILE).
  -a, --account=STRING               Account used to execute command ($UP_ACCOUNT).
      --insecure-skip-tls-verify     [INSECURE] Skip verifying TLS certificates ($UP_INSECURE_SKIP_TLS_VERIFY).

  -d, --description=STRING           Description for control plane.

$ up alpha ctp create cool-plane-2
dan/cool-plane-2 created

```

This strategy may also be implemented in the future when promoting commands from
a lower level of maturity to a higher level, but it is encouraged for users to
_never_ depend on hidden commands. This functionality is only provided as a
convenience for users to extend the window in which they must update their
systems to the new commands.
