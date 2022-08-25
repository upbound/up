# Commands

Commands implemented by `up` are categorized into groups with increasing levels
of specificity. This `up <noun-0>...<noun-N> <verb> [args...]` style caters to
simple discovery and a clearer mapping to REST API endpoints with which the CLI
interacts.

Groups:
- [Top-Level](#top-level)
- [Control Plane](#control-plane)
- [Organization](#organization)
- [Repository](#repository)
- [Robot](#robot)
- [UXP](#uxp)
- [XPKG](#xpkg)
- [XPLS](#xpls)
- [Alpha](#upbound)

## Top-Level

Top-level commands do not belong in any subgroup, and are generally used to
configure `up` itself.

Format: `up <cmd> ...`

- `license`
    - Behavior: Prints license information for the `up` binary, which is under
      the [Upbound Software License].
- `login`
    - Flags:
        - `-p,--password = STRING` (Env: `UP_PASS`): Password for specified
          user. If `-` is given the value will be read from stdin.
        - `-t,--token = STRING` (Env: `UP_TOKEN`): Upbound API token used to
          perform the login. If `-` is given the value will be read from stdin.
        - `-u,--username = STRING` (Env: `UP_USER`): User with which to perform
          the login. Email can also be used as username.
        - `--domain = URL` (Env: `UP_DOMAIN`) (Default: `https://upbound.io`):
          Endpoint to use when communicating with the Upbound API.
        - `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to
          perform the specified command.
        - `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to
          perform the specified command. Can be either an organization or a
          personal account.
    - Behavior: Acquires a session token based on the provided information. If
      only username is provided, the user will be prompted for a password. If
      neither username or password is provided, the user will be prompted for
      both. If token is provided, the user will not be prompted for input. The
      acquired session token will be stored in `~/.up/config.json`. Interactive
      input is disabled if stdin is not an interactive terminal.
- `logout`
    - Flags:
        - `--domain = URL` (Env: `UP_DOMAIN`) (Default: `https://upbound.io`):
          Endpoint to use when communicating with the Upbound API.
        - `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to
          perform the specified command.
       - `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to
          perform the specified command. Can be either an organization or a
          personal account.
        - `--insecure-skip-tls-verify = BOOL` (Env:
          `UP_INSECURE_SKIP_TLS_VERIFY`): Skip verifying TLS certificates.
    - Behavior: Invalidates the session token for the default profile or one
      specified with `--profile`.

**Flags:**

Top-level flags can be passed for any top-level or group-specific command.

- `-h,--help`: Print help and exit.
- `-v,--version`: Print current `up` version and exit.
- `-q,--quiet`: Suppresses all output.
- `--pretty`: Pretty prints output.

## Control Plane

Format: `up controlplane <cmd> ...` Alias: `up ctp <cmd> ...`

Commands in the **Control Plane** group are used to manage and interact with
control planes.

**Group Flags**

Group flags can be passed for any command in the **Control Plane** group. Some
commands may choose not to utilize the group flags when not relevant.

- `--domain = URL` (Env: `UP_DOMAIN`) (Default: `https://upbound.io`): Endpoint
  to use when communicating with the Upbound API.
- `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to perform the
  specified command.
- `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to perform the
  specified command. Can be either an organization or a personal account.
- `--insecure-skip-tls-verify = BOOL` (Env: `UP_INSECURE_SKIP_TLS_VERIFY`): Skip
  verifying TLS certificates.

**Subgroup: Pull Secret**

Format: `up controlplane pull-secret <cmd> ...` Alias: `up ctp pull-secret
<cmd>...`

- `create <control-plane-ID>`
    - Flags:
        - `-f,--file = FILE`: Path to token credentials file. Credentials from
          profile are used if not specified.
        - `--kubeconfig = STRING`: sets `kubeconfig` path. Same defaults as
        `kubectl` are used if not provided.
        - `-n,--namespace = STRING` (Env: `UPBOUND_NAMESPACE`) (Default:
          `upbound-system`): Kubernetes namespace used for installing and
          managing Upbound.
    - Behavior: Creates a package pull secret in the cluster pointed to by the
      currently configured `kubeconfig`. If `-f` is not provided, the session
      token in the current profile is used. Session tokens expire within 30
      days. If `-f` is provided, it must point to a file with valid token
      credentials in the following JSON format: `{"accessId": "<access-id>",
      "token":"<token>"}`. This is the same format emitted by `up robot token
      create`. Robot tokens do not expire.

## Profile

Format: `up profile <cmd> ...` 

Commands in the **Profile** group are used to configure `up` using named
profiles. See the [configuration documentation] for more information on how
Upbound profiles are structured.

- `current`
    - Behavior: Gets the current Upbound profile. Sensitive data is obfuscated.
- `list`
    - Behavior: Lists all Upbound profiles.
- `use <name>`
    - Behavior: Sets the default Upbound profile to the one specified by the
      provided name.
- `view`
    - Behavior: Gets all Upbound profiles. Sensitive data is obfuscated.

**Group Flags**

Group flags can be passed for any command in the **Profile** group. Some
commands may choose not to utilize the group flags when not relevant.

- `--domain = URL` (Env: `UP_DOMAIN`) (Default: `https://upbound.io`): Endpoint
  to use when communicating with the Upbound API.
- `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to perform the
  specified command.
- `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to perform the
  specified command. Can be either an organization or a personal account.
- `--insecure-skip-tls-verify = BOOL` (Env: `UP_INSECURE_SKIP_TLS_VERIFY`): Skip
  verifying TLS certificates.

**Subgroup: Config**

Format: `up profile config <cmd> ...`

- `set [<key> [<value>]]`
    - Flags:
        - `-f, --file = FILE`: Path to configuration file. Must be in JSON
          format.
    - Behavior: Adds the provided key, value pair(s) to the Upbound profile.
- `unset [<key>]`
    - Flags:
        - `-f, --file = FILE`: Path to configuration file. Must be in JSON
          format.
    - Behavior: Removes the provided key, value pair(s) from the Upbound
      profile.

## Organization

Format: `up organization <cmd> ...` Alias: `up org <cmd>...`

Commands in the **Organization** group are used to interact with Upbound
organizations.

- `create <name>`
    - Behavior: Creates an Upbound organization.
- `delete <name>`
    - Flags:
        - `--force = BOOL`: Force deletion of organization.
    - Behavior: Deletes an Upbound organization.
- `list`
    - Behavior: Lists all Upbound organizations in which the user is a member or
      owner.

**Group Flags**

Group flags can be passed for any command in the **Organization** group. Some
commands may choose not to utilize the group flags when not relevant.

- `--domain = URL` (Env: `UP_DOMAIN`) (Default: `https://upbound.io`): Endpoint
  to use when communicating with the Upbound API.
- `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to perform the
  specified command.
- `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to perform the
  specified command. Can be either an organization or a personal account.
- `--insecure-skip-tls-verify = BOOL` (Env: `UP_INSECURE_SKIP_TLS_VERIFY`): Skip
  verifying TLS certificates.

## Repository

Format: `up repository <cmd> ...` Alias: `up repo <cmd>...`

Commands in the **Repository** group are used to interact with repositories in
Upbound accounts.

- `create <name>`
    - Behavior: Creates a repository with the specified name in the current
      account.
- `delete <name>`
    - Flags:
        - `--force = BOOL`: Force deletion of repository.
    - Behavior: Deletes the repository with the specified name in the current
      account.
- `list`
    - Behavior: Lists all repositories in the current account.

**Group Flags**

Group flags can be passed for any command in the **Organization** group. Some
commands may choose not to utilize the group flags when not relevant.

- `--domain = URL` (Env: `UP_DOMAIN`) (Default: `https://upbound.io`): Endpoint
  to use when communicating with the Upbound API.
- `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to perform the
  specified command.
- `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to perform the
  specified command. Can be either an organization or a personal account.
- `--insecure-skip-tls-verify = BOOL` (Env: `UP_INSECURE_SKIP_TLS_VERIFY`): Skip
  verifying TLS certificates.

## Robot

Format: `up robot <cmd> ...`

Commands in the **Robot** group are used to interact with robot accounts in
Upbound organizations.

- `create <name>`
    - Behavior: Creates a robot account with the specified name in the current
      organization.
- `delete <name>`
    - Flags:
        - `--force = BOOL`: Force deletion of robot.
    - Behavior: Deletes the robot with the specified name in the current
      organization.
- `list`
    - Behavior: Lists all robots in the current organization.

**Group Flags**

Group flags can be passed for any command in the **Organization** group. Some
commands may choose not to utilize the group flags when not relevant.

- `--domain = URL` (Env: `UP_DOMAIN`) (Default: `https://upbound.io`): Endpoint
  to use when communicating with the Upbound API.
- `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to perform the
  specified command.
- `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to perform the
  specified command. Can be either an organization or a personal account.
- `--insecure-skip-tls-verify = BOOL` (Env: `UP_INSECURE_SKIP_TLS_VERIFY`): Skip
  verifying TLS certificates.

**Subgroup: Token**

Format: `up robot token <cmd> ...`

- `create <robot-name> <token-name>`
    - Flags:
        - `-o,--output = FILE` (*Required*): Path to file for writing token
          credentials. If `-` is provided, credentials will be printed to the
          terminal.
    - Behavior: Creates a token with the specified name for the specified robot
      account in the current organization.
- `delete <robot-name> <token-name>`
    - Flags:
        - `--force = BOOL`: Force deletion of token.
    - Behavior: Deletes the token with the specified name for the specified
      robot account in the current organization.
- `list <robot-name>`
    - Behavior: Lists all tokens for the specified robot account in the current
      organization.

## UXP

Format: `up uxp <cmd> ...`

Commands in the **UXP** group are used to install and manage Upbound Universal
Crossplane.

- `install [version]`
    - Flags:
        - `--unstable = BOOL`: Allows installing unstable UXP versions. If using
          Helm as install engine, setting to `true` will use
          https://charts.upbound.io/main as repository instead of
          https://charts.upbound.io/stable.
        - `--set = KEY=VALUE`: Set install parameters for UXP. Flag can be
          passed multiple times and multiple key-value pairs can be provided in
          a comma-separated list.
        - `-f,--file = FILE`: YAML file with parameters for UXP install. Follows
          format of Helm-style values file.
    - Behavior: Installs UXP into cluster specified by currently configured
      `kubeconfig`. When using Helm as install engine, the command mirrors the
      behavior of `helm install`. If `[version]` is not provided, the latest
      chart version will be used from the either the stable or unstable
      repository.
- `upgrade [version]` 
    - Flags:
        - `--rollback = BOOL`: Indicates that the upgrade should be rolled back
          in case of failure.
        - `--unstable = BOOL`: Allows installing unstable UXP versions. If using
          Helm as install engine, setting to `true` will use
          https://charts.upbound.io/main as repository instead of
          https://charts.upbound.io/stable.
        - `--set = KEY=VALUE`: Set install parameters for UXP. Flag can be
          passed multiple times and multiple key-value pairs can be provided in
          a comma-separated list.
        - `-f,--file = FILE`: YAML file with parameters for UXP install. Follows
          format of Helm-style values file.
        - `--force = BOOL`: Forces upgrade even if versions are incompatible.
          This is only relevant when upgrading from Crossplane to UXP. 
    - Behavior: Upgrades UXP in cluster specified by currently configured
      `kubeconfig` in the specified namespace. If `[version]` is not provided,
      the latest chart version will be used from the either the stable or
      unstable repository. This command can also be used to upgrade a currently
      installed Crossplane version to a _compatible UXP version_ (i.e. one that
      has the same major, minor, and patch version). The following scenarios are
      supported when upgrading from Crossplane `vX.Y.Z` installed in the
      `crossplane-system` namespace:
        - `up uxp upgrade vX.Y.Z-up.N -n crossplane-system`: upgrades Crossplane
          to compatible UXP version in the same namespace.
        - `up uxp upgrade vA.B.C-up.N -n crossplane-system --force`: upgrades
          Crossplane to an incompatible UXP version in the same namespace.
        - `up uxp upgrade -n crossplane-system --force`: upgrades Crossplane to
          an incompatible latest stable version of UXP in the same namespace.
        - `up uxp upgrade vX.Y.Z-up.N.rc.xyz -n crossplane-system --unstable`:
          upgrades Crossplane to a compatible unstable version of UXP in the
          same namespace.
        - `up uxp upgrade vA.B.C-up.N.rc.xyz -n crossplane-system --unstable
          --force`: upgrades Crossplane to a incompatible unstable version of
          UXP in the same namespace.
        - `up uxp upgrade -n crossplane-system --unstable --force`: upgrades
          Crossplane to an incompatible latest unstable version of UXP in the
          same namespace.
- `uninstall` 
    - Behavior: Uninstalls UXP from the cluster specified by currently
      configured `kubeconfig`.

**Group Flags**

Group flags can be passed for any command in the **UXP** group. Some commands
may choose not to utilize the group flags when not relevant.

- `--kubeconfig = STRING`: sets `kubeconfig` path. Same defaults as `kubectl`
  are used if not provided.
- `-n,--namespace = STRING` (Env: `UXP_NAMESPACE`) (Default: `upbound-system`):
  Kubernetes namespace used for installing and managing UXP.

## XPKG

Format: `up xpkg <cmd> ...`

Commands in the **XPKG** group are used to build, push, and interact with
on-disk Crossplane packages.

- `build`
    - Flags:
        - `--name = STRING` (*DEPRECATED: use `--output` instead.*): Name of the
          package to be built. Uses name in `crossplane.yaml` if not specified.
          Does not correspond to package tag.
        - `-o,--output = FILE`: Path to out file for package. Uses name in
          `crossplane.yaml` and root package directory if not specified. Does
          not correspond to package tag.
        - `--controller = STRING`: Controller image to use as base when
          constructing bundled Provider packages. Image must be available in
          local Docker daemon.
        - `-f,--package-root = STRING`: Path to package directory.
        - `-e,--examples-root = STRING` (Default: `./examples`): Path to package
          examples directory.
        - `--ignore = STRING,...`: Paths, specified relative to --package-root,
          to exclude from the package.
    - Behavior: Builds a Crossplane package (`.xpkg`) that is compatible with
      upstream Crossplane packages and is a valid OCI image. Build will fail if
      package is malformed or contains resources that are not compatible with
      its type (e.g. a `Provider` package containing a `Composition`).
- `init`
    - Flags:
        - `-p,--package-root = STRING` (Default: `.`): Path to directory where
          package will be initialized.
        - `-t,--type = STRING` (Default: `configuration`): Type of package to
          initialize.
    - Behavior: Initializes a package in the specified directory.
- `dep [package]`
    - Flags:
        - `--cache-dir = STRING` (Default: `~/.up/cache`): Path to package
          dependency cache.
        - `-c,--clean-cache = BOOL`: Clean the dependency cache.
    - Behavior: Adds a package to the dependency cache.
- `push <tag>`
    - Flags:
        - `-f,--package = STRING`: Path to package. If not specified and only
          one package exists in current directory it will be used.
        - `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to
          perform the specified command.
    - Behavior: Pushes a Crossplane package (`.xpkg`) to an OCI compliant
      registry. The [Upbound Marketplace] (`xpkg.upbound.io`) will be used by
      default if tag does not specify.

## XPLS

Format: `up xpls <cmd> ...`

Commands in the **XPLS** group are used to interact with the Crossplane language
server. 

- `serve`
    - Flags:
        - `--cache = STRING` (Default: `~/.up/cache`): Path to package cache.
        - `--verbose = BOOL`: Run server with verbose logging.
    - Behavior: Runs the Crossplane language server.

## Alpha

The **Alpha** group includes commands that are _alpha_ maturity level. These
commands may or may not be present in subsequent releases of `up`.

### Control Plane

Format: `up alpha controlplane <cmd> ...` Alias: `up alpha ctp <cmd> ...`

Commands in the **Control Plane** group are used to manage and interact with
control planes.

- `create <name>`
    - Flags:
        - `--description = STRING`: Control plane description.
    - Behavior: Creates a hosted control plane in Upbound.
- `delete <id>`
    - Behavior: Deletes a control plane in Upbound.
- `list`
    - Behavior: Lists all control planes for the configured account.

**Group Flags**

Group flags can be passed for any command in the **Control Plane** group. Some
commands may choose not to utilize the group flags when not relevant.

- `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to perform the
  specified command. Can be either an organization or a personal account.
- `--endpoint = URL` (Env: `UP_DOMAIN`) (Default: `https://api.upbound.io`):
  Endpoint to use when communicating with the Upbound API.
- `--mcp-experimental = BOOL` (Env: `UP_MCP_EXPERIMENTAL`): Use experimental
  control planes API.
- `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to perform the
  specified command.

**Subgroup: Kubeconfig**

Format: `up alpha controlplane kubeconfig <cmd> ...` Alias: `up ctp kubeconfig
<cmd>...`

- `get <control-plane-ID>`
    - Flags:
        - `--token = STRING` (*Required*): API token for authenticating to
          control plane. If `-` is given the value will be read from stdin.
        - `-f,--file = FILE`: File to merge `kubeconfig`.
        - `--proxy = URL` (Env: `UP_PROXY`) (Default:
          `https://proxy.upbound.io/env`): Endpoint for Upbound control plane
          proxy.
    - Behavior: Merges control plane cluster and authentication data into
      currently configured `kubeconfig`, or one specified by `--file`. The
      `--token` flag must be provided and must be a valid Upbound API token. A
      new context will be created for the cluster and authentication data and it
      will be set as current.

### Upbound

Format: `up upbound <cmd> ...`

Commands in the **Upbound** group are used to install and manage Upbound.
Installing and upgrading Upbound requires an Upbound customer license. Users
will be prompted for their License ID and License Key on installation.

- `install <version>`
    - Flags:
        - `--license-secret-name = STRING` (Default: `upbound-license`): Allows
          setting the name of the license `Secret` that is created on
          installation. The default value is expected, so passing an alternate
          value for this flag usually requires modifying the installation
          configuration using one of the following flags.
        - `--set = KEY=VALUE`: Set install parameters for Upbound. Flag can be
          passed multiple times and multiple key-value pairs can be provided in
          a comma-separated list.
        - `-f,--file = FILE`: YAML file with parameters for Upbound install.
          Follows format of Helm-style values file.
        - `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to
          perform the specified command. Can be either an organization or a
          personal account.
        - `--endpoint = URL` (Env: `UP_DOMAIN`) (Default:
          `https://api.upbound.io`): Endpoint to use when communicating with the
          Upbound API.
        - `--mcp-experimental = BOOL` (Env: `UP_MCP_EXPERIMENTAL`): Use
          experimental control planes API.
        - `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to
          perform the specified command.
    - Behavior: Installs Upbound into cluster specified by currently configured
      `kubeconfig`. When using Helm as install engine, the command mirrors the
      behavior of `helm install`. If `[version]` is not provided, the latest
      chart version will be used from the either the stable or unstable
      repository.
- `upgrade <version>` 
    - Flags:
        - `--rollback = BOOL`: Indicates that the upgrade should be rolled back
          in case of failure.
        - `--set = KEY=VALUE`: Set install parameters for Upbound. Flag can be
          passed multiple times and multiple key-value pairs can be provided in
          a comma-separated list.
        - `-f,--file = FILE`: YAML file with parameters for Upbound install.
          Follows format of Helm-style values file.
    - Behavior: Upgrades Upbound in cluster specified by currently configured
      `kubeconfig` in the specified namespace.
- `uninstall` 
    - Behavior: Uninstalls Upbound from the cluster specified by currently
      configured `kubeconfig`.
- `mail`
    - Flags:
        - `-p,--port = INT` (Default: `8085`): Port used for mail portal.
        - `--verbose = BOOL`: Run server with verbose logging.
    - Behavior: Runs a local mail portal for Upbound when configured to send
      emails as Kubernetes Secrets.

**Group Flags**

Group flags can be passed for any command in the **Upbound** group. Some
commands may choose not to utilize the group flags when not relevant.

- `--kubeconfig = STRING`: sets `kubeconfig` path. Same defaults as `kubectl`
  are used if not provided.
- `-n,--namespace = STRING` (Env: `UPBOUND_NAMESPACE`) (Default:
  `upbound-system`): Kubernetes namespace used for installing and managing
  Upbound.

### XPKG

Format: `up alpha xpkg <cmd> ...`

Commands in the **XPKG** group are used to build, push, and interact with
on-disk Crossplane packages.

- `xp-extract <package>`
    - Flags:
        - `--from-daemon = BOOL`: Indicates that the image should be fetched
          from the Docker daemon instead of the registry.
        - `-o, --output = STRING` (Default: `out.gz`): Package output file path.
          Extension must be .gz or will be replaced
    - Behavior: Extract package contents into a Crossplane cache compatible
      format. `package` must be a valid OCI image reference and is fetched from
      a remote registry unless `--from-daemon` is specified. The [Upbound
      Registry] (`xpkg.upbound.io`) will be used by default if reference does
      not specify.

<!-- Named Links -->
[Upbound Software License]: https://licenses.upbound.io/upbound-software-license.html
[Upbound Marketplace]: https://www.upbound.io/registry
[configuration documentation]: configuration.md
