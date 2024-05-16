# up - The Upbound CLI

`up` is the official CLI for interacting with [Upbound] and [Universal Crossplane (UXP)]. 

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

#### Installing latest development build
```
curl -sL https://cli.upbound.io | CHANNEL=main sh
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

## Usage

Refer to the documentation on [docs.upbound.io] for more information.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

<!-- Named Links -->
[Upbound]: https://console.upbound.io/
[Universal Crossplane (UXP)]: https://github.com/upbound/universal-crossplane
[UXP]: https://github.com/upbound/universal-crossplane
[credential helper protocol]: https://github.com/docker/docker-credential-helpers
[nixpkgs]: https://github.com/NixOS/nixpkgs
[docs.upbound.io]: https://docs.upbound.io/reference/cli/
