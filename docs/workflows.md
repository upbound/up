# Workflows

This document outlines common workflows of `up` users while using the CLI. It
serves as light documentation for users, but its primary purpose is to guide
design decisions and feature implementations based on common user experience.

## Upbound Login

> Though a variety of login methods are demonstrated below, users are highly
> encouraged to provide sensitive data either interactively or by reading from
> stdin.

Interactively be prompted for username and password:

```
$ up login
```

Interactively be prompted for password:

```
$ up login -u hasheddan
```

Login with sensitive data from file:

```
cat password.txt | up login -u hasheddan -p -
```

```
cat token.txt | up login -t -
```

Login with data from environment variables:

```
export UP_USER=hasheddan
export UP_PASSWORD=supersecret

up login -u hasheddan
```

```
export UP_TOKEN=supersecrettoken

up login
```

Login with username and password:

```
$ up login --username=hasheddan --password=supersecret
```

Login with API token:

```
$ up login --token=supersecrettoken
```

Login with specified profile name:

```
$ up login --profile=dev
```

```
$ up login --username=hasheddan --password=supersecret --profile=dev
```

```
$ up login --token=supersecrettoken --profile=dev
```

## Universal Crossplane

`up` can be used to manage the lifecycle of an [Upbound Universal Crossplane]
installation.

### Installing

Install latest stable version:

```
$ up uxp install
```

Install latest unstable version (i.e. development build):

```
$ up uxp install --unstable
```

Install specific stable version:

```
$ up uxp install v1.2.1-up.2
```

Install specific unstable version:

```
$ up uxp install v1.2.1-up.2.rc.0.7-g46c7750 --unstable
```

Install with inline parameters:

```
$ up uxp install --set key1=value1 --set key2=value2
```

Install with parameters file:

```
$ up uxp install -f uxp-params.yaml
```

### Upgrading Crossplane to UXP

`up` supports upgrading a Crossplane installation to a compatible UXP version.
Compatibility is defined as having matching major, minor, and patch versions (in
accordance with [semantic versioning]). In addition, UXP must be installed in
the same namespace where Crossplane is currently installed. Crossplane is
typically installed in the `crossplane-system` namespace.

Upgrade Crossplane vX.Y.Z to UXP:

```
$ up uxp upgrade vX.Y.Z-up.N -n crossplane-system
```

> Because installations cannot be moved from one namespace to another, users
> operating outside of the default `upbound-system` namespace will frequently
> set `UXP_NAMESPACE=<namespace>` to avoid having to supply it for every UXP
> command.


<!-- Named Links -->
[Upbound Universal Crossplane]: https://github.com/upbound/universal-crossplane
[semantic versioning]: https://semver.org/
