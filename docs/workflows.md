# Workflows

This document outlines common workflows of `up` users while using the CLI. It
serves as light documentation for users, but its primary purpose is to guide
design decisions and feature implementations based on common user experience.

## Upbound Cloud Login

> Though a variety of login methods are demonstrated below, users are highly
> encouraged to provide sensitive data either interactively or by reading from
> stdin.

Interactively be prompted for username and password:

```
$ up cloud login
```

Interactively be prompted for password:

```
$ up cloud login -u hasheddan
```

Login with sensitive data from file:

```
cat password.txt | up cloud login -u hasheddan -p -
```

```
cat token.txt | up cloud login -t -
```

Login with data from environment variables:

```
export UP_USER=hasheddan
export UP_PASSWORD=supersecret

up cloud login -u hasheddan
```

```
export UP_TOKEN=supersecrettoken

up cloud login
```

Login with username and password:

```
$ up cloud login --username=hasheddan --password=supersecret
```

Login with API token:

```
$ up cloud login --token=supersecrettoken
```

Login with specified profile name:

```
$ up cloud login --profile=dev
```

```
$ up cloud login --username=hasheddan --password=supersecret --profile=dev
```

```
$ up cloud login --token=supersecrettoken --profile=dev
```

## Hosted Control Plane

Create hosted control plane on Upbound Cloud:

```
$ up cloud controlplane create my-hosted-cp
```

```
$ up cloud xp create my-hosted-cp
```

## Self-Hosted Control Plane

Creating a self-hosted control plane on Upbound Cloud consists of three primary
steps: installing UXP, creating a self-hosted control plane on Upbound Cloud
(i.e. "attaching"), and connecting UXP to that control plane.

### Installing UXP

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

`up` also supports upgrading a Crossplane installation to a compatible UXP
version. Compatibility is defined as having matching major, minor, and patch
versions (in accordance with [semantic versioning]). In addition, UXP must be
installed in the same namespace where Crossplane is currently installed.
Crossplane is typically installed in the `crossplane-system` namespace.

Upgrade Crossplane vX.Y.Z to UXP:

```
$ up uxp upgrade vX.Y.Z-up.N -n crossplane-system
```

> Because installations cannot be moved from one namespace to another, users
> operating outside of the default `upbound-system` namespace will frequently
> set `UXP_NAMESPACE=<namespace>` to avoid having to supply it for every UXP
> command.

### Attaching a Self-Hosted Control PLane

Attach a self-hosted control plane on Upbound Cloud:

```
$ up cloud controlplane attach my-self-hosted-cp
<control-plane-token>
```

```
$ up cloud xp attach my-self-hosted-cp
<control-plane-token>
```

Self-hosted control planes can be created with "view only" permissions:

```
$ up cloud xp attach my-self-hosted-cp --view-only
<control-plane-token>
```

### Connecting UXP to a Self-Hosted Control Plane

Connect UXP to self-hosted control plane:

```
$ up uxp connect <control-plane-token>
```

Most users pipe the attach command into the connect one:

```
$ up cloud controlplane attach my-self-hosted-cp | up uxp connect -
```

```
$ up cloud xp attach my-self-hosted-cp | up uxp connect -
```

<!-- Named Links -->
[semantic versioning]: https://semver.org/
