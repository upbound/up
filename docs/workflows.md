# Workflows

This document outlines common workflows of `up` users while using the CLI. It
serves as light documentation for users, but its primary purpose is to guide
design decisions and feature implementations based on common user experience.

## Upbound Cloud Login

Login with username and password:

```
$ up cloud login --username=hasheddan --password=supersecret
```

Login with API token:

```
$ up cloud login --token=supersecrettoken
```

## Hosted Control Plane

Create hosted control plane on Upbound Cloud:

```
$ up cloud controlplane create my-hosted-cp
```

Fetch `kubeconfig` for hosted control plane:

```
$ up cloud controlplane kubeconfig my-hosted-cp > kube.yaml
```

Interact with hosted control plane:

```
$ kubectl --kubeconfig=kube.yaml get providers
```

## Self-Hosted Control Plane

Install UXP into any Kubernetes cluster:

```
$ up uxp install
```

Create self-hosted control plane on Upbound Cloud:

```
$ up cloud controlplane attach my-self-hosted-cp
<control-plane-token>
```

Connect UXP to self-hosted control plane:

```
$ up uxp connect my-self-hosted-cp --cp-token=<control-plane-token>
```

Most users will likely pipe the attach command into the connect one:

```
$ up cloud controlplane attach my-self-hosted-cp | up uxp connect my-self-hosted-cp --cp-token=-
```
