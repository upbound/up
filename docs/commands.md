# Commands

Commands implemented by `up` are categorized into groups with increasing levels
of specificity. This `up <noun-0>...<noun-N> <verb> [args...]` style caters to
simple discovery and a clearer mapping to REST API endpoints with which the CLI
interacts.

## Top-Level

**Commands**

Top-level commands do not belong in any subgroup, and are generally used to
configure `up` itself.

Format: `up <cmd> ...`

- `login`
    - Status: `Unimplemented`
    - Flags:
        - `-p,--password = STRING` (Env: `UP_PASS`): Password for specified
          user.
    - Behavior: Acquires a session token based on the provided information. If
      no information is provided, the user will be interactively prompted for a
      username first, then a password. If only a username is provided, the user
      will be interactively prompted for a password. If both username and
      password are provided, there will be no prompt. The acquired session token
      will be stored in `~/.up/config.json` alongside the username.

**Flags:**

Top-level flags can be passed for any top-level or group-specific command. Some
commands may choose not to utilize top-level flags when not relevant.

- `-h, --help`: Print help and exit.
- `-o, --organization = STRING` (Env: `UP_ORG`): Organization with which to
  perform the specified command.
- `-u,--username = STRING` (Env: `UP_USER`): User with which to perform the
  specified command. User must already be present in `config.json` unless the
  command involves logging in. Email can also be used as username.
- `-v,--version`: Print current `up` version and exit.

## Default

Commands in the **Default** group are used to set global defaults for `up`.

Format: `up default [subgroups...] <cmd> ...`

- `set`
    - Status: `Unimplemented`
    - Behavior: Interactively prompts for all available `up` defaults. Defaults
      are stored in `config.json`.

### Subgroup: Organization

Format: `up default organization <cmd> ...` Alias: `org`

- `set <id>`
    - Status: `Unimplemented`
    - Behavior: Sets default organization `id` in `config.json`. Default
      organization can also be set to username to make operations default to the
      user account rather than an actual organization.

### Subgroup: User

Format: `up default user <cmd> ...`

- `set <username>`
    - Status: `Unimplemented`
    - Behavior: Sets default user `username` in `config.json`.

## Control Plane

Format: `up controlplane <cmd> ...` Alias: `up cp <cmd> ...`

- `create <name>`
    - Status: `Unimplemented`
    - Flags:
        - `--self-hosted = BOOL` (Default: `False`): Indicates whether the
          control plane is self-hosted.
        - `--kube-cluster-id = STRING`: UID for self-hosted control plane. Only
          required if `--self-hosted=true` and will be auto-populated as
          `metadata.uid` of `kube-system` `Namespace` of currently configured
          `kubeconfig` if not manually provided.
        - `--description = STRING`: Control plane description.
        - `--install-options = STRING`: Accepts and passes along any Helm
          `install` flags provided. Only honored when control plane is
          self-hosted.
    - Behavior: Creates a control plane on Upbound Cloud. Control plane may be
      hosted or self-hosted. If self-hosted and UXP is already installed, the
      cluster will be automatically connected to Upbound Cloud (see `up uxp
      connect`). If self-hosted and UXP is not already installed, it will be
      installed into the cluster (see `up uxp install`).
- `delete <name>`
    - Status: `Unimplemented`
    - Behavior: Deletes a control plane on Upbound Cloud. If control plane is
      hosted, the UXP cluster will be deleted. If the control plane is
      self-hosted, the UXP cluster will begin failing to connect to Upbound
      Cloud.

## UXP

Format: `up uxp <cmd> ...`

Commands in the **UXP** group are used to install and manage Upbound Universal
Crossplane.

- `install [version]`
    - Status: `Unimplemented`
    - Flags:
        - `--options = STRING`: Accepts and passes along any Helm `install`
          flags provided.
    - Behavior: Installs UXP into cluster specified by currently configured
      `kubeconfig`. Essentially `helm install` but does not require specifying
      chart. If `version` is not provided, the latest chart version will be
      used.
- `connect <name>`
    - Status: `Unimplemented`
    - Behavior: Connects the UXP instance in cluster specified by currently
      configured `kubeconfig` to the existing self-hosted control plane
      specified by `name`.
- `upgrade [version]` 
    - Status: `Unimplemented`
    - Flags:
        - `--options = STRING`: Accepts and passes along any Helm `upgrade`
          flags provided.
    - Behavior: Upgrades UXP in cluster specified by currently configured
      `kubeconfig`. Essentially `helm upgrade` but does not require specifying
      chart. If `version` is not provided, the latest chart version will be
      used.
- `uninstall` 
    - Status: `Unimplemented`
    - Flags:
        - `-f,--force = BOOL` (Default: `False`): Indicates that UXP should be
          uninstalled without warning. This may result in deletion or orphaning
          of external infrastructure.
    - Behavior: Uninstalls UXP from the cluster specified by currently
      configured `kubeconfig`.

## Organization

Format: `up organization <cmd> ...` Alias: `org`

Commands in the **Organization** group are used to interact with organizations
on Upbound Cloud.

- `create <id> <name>`
    - Status: `Unimplemented`
    - Behavior: Creates a new organization with provided `id` and `name`.
- `delete <id>`
    - Status: `Unimplemented`
    - Behavior: Deletes an organization with provided `id`.
- `list`
    - Status: `Unimplemented`
    - Behavior: Lists all organizations accessible by the currently
      authenticated user.

## Provider

Format: `up provider <cmd> ...`

Commands in the **Provider** group are used to interact with Providers in a UXP
cluster.

- `install <package> [name]`
    - Status: `Unimplemented`
    - Behavior: Creates a `Provider` instance in the UXP cluster with specified
      `package`. If `name` is not provided, the name of the image (with registry
      identifier stripped) will be used. 
- `upgrade <name> <version>`
    - Status: `Unimplemented`
    - Behavior: Upgrades the `Provider` instance with `name` in the UXP cluster
      to the specified `version`.
- `uninstall <name>`
    - Status: `Unimplemented`
    - Behavior: Delete the `Provider` instance with `name` in the UXP cluster.
- `list`
    - Status: `Unimplemented`
    - Behavior: Lists all `Provider` instances in the UXP cluster.

## Configuration

Format: `up configuration <cmd> ...`

Commands in the **Configuration** group are used to interact with Configurations
in a UXP cluster.

- `install <package> [name]`
    - Status: `Unimplemented`
    - Behavior: Creates a `Configuration` instance in the UXP cluster with
      specified `package`. If `name` is not provided, the name of the image
      (with registry identifier stripped) will be used. 
- `upgrade <name> <version>`
    - Status: `Unimplemented`
    - Behavior: Upgrades the `Configuration` instance with `name` in the UXP
      cluster to the specified `version`.
- `uninstall <name>`
    - Status: `Unimplemented`
    - Behavior: Delete the `Configuration` instance with `name` in the UXP
      cluster.
- `list`
    - Status: `Unimplemented`
    - Behavior: Lists all `Configuration` instances in the UXP cluster.

## Package

Format: `up package <cmd> ...` Alias: `pkg`

Commands in the **Package** group are used to develop, build, and push
Crossplane packages.

- `build [name]`
    - Status: `Unimplemented`
    - Flags:
        - `-f,--package-root = STRING`: Path to root package directory, relative
          to current working directory.
        - `--ignore = []STRING`: List of Go style path pattern matches to
          exclude from package.
    - Behavior: Builds the Configuration or Provider package in the current
      working directory. If `name` is provided, it will be used in output as
      `<name>.xpkg`. Otherwise, the `metadata.name` in the `crossplane.yaml`
      will be used. If `--package-root` is provided it will be used as working
      directory. Any paths that match patterns passed to `--ignore` will be
      excluded from the package. Files without `.yaml` or `.yml` extensions will
      be automatically excluded.
- `push <name>`
    - Status: `Unimplemented`
    - Flags:
        - `-f,--package = STRING`: Path to package file.
    - Behavior: Builds the Configuration or Provider package in the current
      working directory. If `name` is provided, it will be used in output as
      `<name>.xpkg`. Otherwise, the `metadata.name` in the `crossplane.yaml`
      will be used. If `--package` is provided it must point to a `.xpkg` file
      and that package will be pushed. If not provided, the `.xpkg` file in the
      working directory will be pushed.
