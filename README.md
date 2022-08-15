# up - The Upbound CLI

<a href="https://upbound.io">
    <img align="right" style="margin-left: 20px" src="docs/media/logo.png" width=200 />
</a>

`up` is the official CLI for interacting with [Upbound Cloud], Upbound
Enterprise, and [Universal Crossplane (UXP)]. It is also the primary tool for
building [Crossplane packages] and pushing them to registries.

For users who wish to use other OCI clients for pushing packages,
`docker-credential-up` is an implementation of the Docker [credential helper
protocol] and can be used as an authentication mechanism for pushing packages by
adding it your Docker config file.

## Install

Both `up` and `docker-credential-up` can be downloaded by using the official
installation script, or can be installed via a variety of common package
managers.

### Install Script:

**up**
```
curl -sL https://cli.upbound.io | sh
```

**docker-credential-up**
```
curl -sL https://cli.upbound.io | BIN=docker-credential-up sh
```

### Homebrew

**up**
```
brew install upbound/tap/up
```

**docker-credential-up**
```
brew install upbound/tap/docker-credential-up
```

### Deb/RPM Packages

Deb and RPM packages are available for Linux platforms, but currently require
manual download and install.

**up**
```
curl -sLo up.deb https://cli.upbound.io/stable/${VERSION}/deb/linux_${ARCH}/up.deb
```

**docker-credential-up**
```
curl -sLo up.deb https://cli.upbound.io/stable/${VERSION}/deb/linux_${ARCH}/docker-credential-up.deb
```

**up**
```
curl -sLo up.rpm https://cli.upbound.io/stable/${VERSION}/rpm/linux_${ARCH}/up.rpm
```

**docker-credential-up**
```
curl -sLo up.rpm https://cli.upbound.io/stable/${VERSION}/rpm/linux_${ARCH}/docker-credential-up.rpm
```

### Nix

The `up` CLI is available via [Nixpkgs] via the `upbound` attribute. To install using
`nix-env`:

```
nix-env -iA upbound
```

To install using the unified `nix` CLI:

```
nix profile install upbound
```

Both installation methods install both the `docker-credential-up` and `up` executables.

## Setup

Users typically begin by either logging in to Upbound or installing [UXP].

### Upbound Login

`up` uses profiles to manage sets of credentials for interacting with [Upbound
Cloud] and Upbound Enterprise. You can read more about how to manage multiple
profiles in the [configuration documentation]. If no `--profile` flag is
provided when logging in the profile designated as default will be updated, and
if no profiles exist a new one will be created with name `default` and it will
be designated as the default profile.

```
up login
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
up uxp upgrade vX.Y.Z-up.N -n <crossplane-namespace>
```

### Build and Push Packages

`up` can be used to build and push Crossplane packages. If pushing to Upbound,
the same credentials acquired via `up login` can be used.

To build a package in your current directory:

```
up xpkg build
```

To push the package:

```
up xpkg push hasheddan/cool-xpkg:v0.1.0
```

If you prefer to use Docker, or any other OCI client, you can add the following
to your config file after downloading `docker-credential-up` to use your Upbound
credentials when pushing.

```json
{
	"credHelpers": {
		"xpkg.upbound.io": "up",
		"registry.upbound.io": "up"
	}
}
```

## Usage

See the documentation on [supported commands] and [common workflows] for more
information.


<!-- Named Links -->
[Upbound Cloud]: https://cloud.upbound.io/
[Universal Crossplane (UXP)]: https://github.com/upbound/universal-crossplane
[UXP]: https://github.com/upbound/universal-crossplane
[Crossplane packages]: https://crossplane.io/docs/v1.7/reference/xpkg.html
[credential helper protocol]: https://github.com/docker/docker-credential-helpers
[configuration documentation]: docs/configuration.md
[Crossplane]: https://crossplane.io
[supported commands]: docs/commands.md
[common workflows]: docs/workflows.md
[nixpkgs]: https://github.com/NixOS/nixpkgs