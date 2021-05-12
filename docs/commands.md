# Commands

Commands implemented by `up` are categorized into groups with increasing levels
of specificity. This `up <noun-0>...<noun-N> <verb> [args...]` style caters to
simple discovery and a clearer mapping to REST API endpoints with which the CLI
interacts.

Groups:
- [Top-Level](#top-level)
- [Cloud](#cloud)
  - [Control Plane](#subgroup-control-plane)
- [UXP](#uxp)

## Top-Level

**Commands**

Top-level commands do not belong in any subgroup, and are generally used to
configure `up` itself.

Format: `up <cmd> ...`

- `license`
    - Status: `Implemented`
    - Behavior: Prints license information for the `up` binary, which is under
      the [Upbound Software License].

**Flags:**

Top-level flags can be passed for any top-level or group-specific command. Some
commands may choose not to utilize top-level flags when not relevant.

- `-h,--help`: Print help and exit.
- `-v,--version`: Print current `up` version and exit.

## Cloud

Format: `up cloud <cmd> ...`

Commands in the **Cloud** group are used to interact with Upbound Cloud.

- `login`
    - Status: `Implemented`
    - Flags:
        - `-p,--password = STRING` (Env: `UP_PASS`): Password for specified
          user.
        - `-t,--token = STRING` (Env: `UP_TOKEN`): Upbound API token used to
          perform the login.
        - `-u,--username = STRING` (Env: `UP_USER`): User with which to perform
          the login. Email can also be used as username.
    - Behavior: Acquires a session token based on the provided information.
      Either username and password can be provided or just a token. The acquired
      session token will be stored in `~/.up/config.json`.

**Group Flags**

Group flags can be passed for any command in the **Cloud** group. Some commands
may choose not to utilize the group flags when not relevant.

- `-a,--account = STRING` (Env: `UP_ACCOUNT`): Account with which to perform the
  specified command. Can be either an organization or a personal account.
- `--endpoint = URL` (Env: `UP_ENDPOINT`) (Default: `https://api.upbound.io`):
  Endpoint to use when communicating with the Upbound API.
- `--profile = STRING` (Env: `UP_PROFILE`); Profile with which to perform the
  specified command.

### Subgroup: Control Plane

Format: `up cloud controlplane <cmd> ...` Alias: `up cloud xp <cmd> ...`

- `attach <name>`
    - Status: `Implemented`
    - Flags:
      - `-d,--description = STRING`: Control plane description.
      - `--kube-cluster-id = UUID`: UUID for self-hosted control plane.
        Auto-populated as `metadata.uid` of `kube-system` `Namespace` of
        currently configured `kubeconfig` if not manually provided.
      - `--kubeconfig` (Env: `KUBECONFIG`): overrides default kubeconfig path
        (`~/.kube/config`).
      - `--view-only`: creates the self-hosted control plane as view only.
    - Behavior: Creates a self-hosted control plane on Upbound Cloud and returns
      token to connect a UXP instance to it.
- `create <name>`
    - Status: `Implemented`
    - Flags:
        - `--description = STRING`: Control plane description.
    - Behavior: Creates a hosted control plane on Upbound Cloud.
- `delete <id>`
    - Status: `Implemented`
    - Behavior: Deletes a control plane on Upbound Cloud. If control plane is
      hosted, the UXP cluster will be deleted. If the control plane is
      self-hosted, the UXP cluster will begin failing to connect to Upbound
      Cloud.
- `list`
    - Status: `Implemented` Behavior: Lists all control planes for the
      configured account.

## UXP

Format: `up uxp <cmd> ...`

Commands in the **UXP** group are used to install and manage Upbound Universal
Crossplane, as well as connect it to Upbound Cloud.

- `install [version]`
    - Status: `Implemented`
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
- `connect <control-plane-token>`
    - Status: `Implemented`
    - Flags:
      - `--token-secret-name` (Default: `upbound-control-plane-token`): Sets the
        name of the secret that will be used to store the control plane token in
        the configured Kubernetes cluster.
    - Behavior: Connects the UXP instance in cluster specified by currently
      configured `kubeconfig` to the existing self-hosted control plane
      specified by `<control-plane-token>`. If `-` is given for
      `<control-plane-token>` the value will be read from stdin.
- `upgrade [version]` 
    - Status: `Implemented`
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
    - Status: `Implemented`
    - Behavior: Uninstalls UXP from the cluster specified by currently
      configured `kubeconfig`.

**Group Flags**

Group flags can be passed for any command in the **UXP** group. Some commands
may choose not to utilize the group flags when not relevant.

- `--kubeconfig` (Env: `KUBECONFIG`): overrides default kubeconfig path
  (`~/.kube/config`).
- `-n,--namespace = STRING` (Env: `UXP_NAMESPACE`) (Default: `upbound-system`):
  Kubernetes namespace used for installing and managing UXP.

<!-- Named Links -->
[Upbound Software License]:
https://licenses.upbound.io/upbound-software-license.html
