# Commands

Commands implemented by `up` are categorized into groups with increasing levels
of specificity. This `up <noun-0>...<noun-N> <verb> [args...]` style caters to
simple discovery and a clearer mapping to REST API endpoints with which the CLI
interacts.

Groups:
- [Top-Level](#top-level)
- [Cloud](#cloud)
  - [Control Plane](#subgroup-control-plane)
  - [User](#subgroup-user)
- [UXP](#uxp)

## Top-Level

**Commands**

Top-level commands do not belong in any subgroup, and are generally used to
configure `up` itself.

Format: `up <cmd> ...`

**Flags:**

Top-level flags can be passed for any top-level or group-specific command. Some
commands may choose not to utilize top-level flags when not relevant.

- `-h, --help`: Print help and exit.
- `-o, --organization = STRING` (Env: `UP_ORG`): Organization with which to
  perform the specified command.
- `-u,--username = STRING` (Env: `UP_USER`): User with which to perform the
  specified command. User must already be present in `config.json` unless the
  command involves logging in. Email can also be used as username.
- `-t,--token = STRING` (Env: `UP_TOKEN`): Upbound API token used to perform the
  specified command. Token must already be present in `config.json` unless the
  command involves logging in.
- `-v,--version`: Print current `up` version and exit.

## Cloud

Format: `up cloud <cmd> ...`

Commands in the **Cloud** group are used to interact with Upbound Cloud.

- `login`
    - Status: `Unimplemented`
    - Flags:
        - `-p,--password = STRING` (Env: `UP_PASS`): Password for specified
          user.
    - Behavior: Acquires a session token based on the provided information. If
      no information is provided, the user will be interactively prompted for a
      username first, then a password. If only a username is provided, the user
      will be interactively prompted for a password. If both username and
      password are provided, or token is provided. there will be no prompt. The
      acquired session token will be stored in `~/.up/config.json` alongside the
      username or token name.

### Subgroup: Control Plane

Format: `up cloud controlplane <cmd> ...` Alias: `up cloud cp <cmd> ...`

- `attach <name>`
    - Status: `Unimplemented`
    - Flags:
      - `--kube-cluster-id = STRING`: UID for self-hosted control plane.
        Auto-populated as `metadata.uid` of `kube-system` `Namespace` of
        currently configured `kubeconfig` if not manually provided.
      - `--description = STRING`: Control plane description.
    - Behavior: Creates a self-hosted control plane on Upbound Cloud and returns
      token to connect a UXP instance to it. If self-hosted control plane
      already exists then only a token will be returned.
- `create <name>`
    - Status: `Unimplemented`
    - Flags:
        - `--description = STRING`: Control plane description.
    - Behavior: Creates a hosted control plane on Upbound Cloud.
- `kubeconfig <name>`
    - Status: `Unimplemented`
    - Behavior: Fetches `kubeconfig` for specified hosted control plane on
      Upbound Cloud.
- `delete <name>`
    - Status: `Unimplemented`
    - Behavior: Deletes a control plane on Upbound Cloud. If control plane is
      hosted, the UXP cluster will be deleted. If the control plane is
      self-hosted, the UXP cluster will begin failing to connect to Upbound
      Cloud.

### Subgroup: Organization

Format: `up cloud organization <cmd> ...` Alias: `org`

- `default <id>`
    - Status: `Unimplemented`
    - Behavior: Sets default organization `id` in `config.json`. Default
      organization can also be set to username to make operations default to the
      user account rather than an actual organization.

### Subgroup: User

Format: `up cloud user <cmd> ...`

- `default <username>`
    - Status: `Unimplemented`
    - Behavior: Sets default user `username` in `config.json`. `username` may
      also be the a token name that is present in the `config.json`.

## UXP

Format: `up uxp <cmd> ...`

Commands in the **UXP** group are used to install and manage Upbound Universal
Crossplane, as well as connect it to Upbound Cloud.

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
    - Flags:
        - `--cp-token = STRING`: Token to connect to specified control plane.
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
