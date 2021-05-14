# up - The Upbound CLI

`up` is the official CLI for interacting with [Upbound Cloud] and [Universal
Crossplane (UXP)].

## Install

`up` can be downloaded by using the official installation script, or can be
installed via a variety of common package managers.

### Install Script:

```
curl -sL https://cli.upbound.io | sh
```

### Homebrew

```
brew install upbound/tap/up
```

### Deb/RPM Packages

Deb and RPM packages are available for Linux platforms, but currently require
manual download and install.

```
curl -sLo up.deb https://cli.upbound.io/stable/${VERSION}/deb/linux_${ARCH}/up.deb
```

```
curl -sLo up.rpm https://cli.upbound.io/stable/${VERSION}/rpm/linux_${ARCH}/up.rpm
```

## Setup

Users typically begin by either logging in to [Upbound Cloud] or installing
[UXP].

### Upbound Cloud Login

`up` uses profiles to manage sets of credentials for interacting with [Upbound
Cloud]. You can read more about how to manage multiple profiles in the
[configuration documentation]. To log in for the first time, use your Upbound
username and password.

```
up cloud login -u <username> -p <password>
```

### Install Universal Crossplane

`up` can install [UXP] into any Kubernetes cluster, or upgrade an existing
[Crossplane] installation to UXP of compatible version. UXP versions with the
same major, minor, and patch number are considered compatible (e.g. `v1.2.1` of
Crossplane is compatible with UXP `v1.2.1-up.N`)

To install the latest stable UXP release:

```
up uxp install
```

To upgrade a Crossplane installation to a compatible UXP version:

```
up uxp install vX.Y.Z-up.N -n <crossplane-namespace>
```

## Usage

See the documentation on [supported commands] and [common workflows] for more
information.


<!-- Named Links -->
[Upbound Cloud]: https://cloud.upbound.io/
[Universal Crossplane (UXP)]: https://github.com/upbound/universal-crossplane
[UXP]: https://github.com/upbound/universal-crossplane
[configuration documentation]: docs/configuration.md
[Crossplane]: https://crossplane.io
[supported commands]: docs/commands.md
[common workflows]: docs/workflows.md