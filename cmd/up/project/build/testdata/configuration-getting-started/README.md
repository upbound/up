# configuration-getting-started
An introductory example to Crossplane and Compositions using provider-nop. This will enable provisioning of several different fake resource types.

This repository contains a reference configuration for [Crossplane](https://crossplane.io). This configuration is built with [provider-nop](https://marketplace.upbound.io/providers/crossplane-contrib/provider-nop), a Crossplane provider that simulates the creation of external resources.


## Overview

This platform offers APIs for setting up a variety of basic resources that mirror what you'd find in a Cloud Service Provider such as AWS, Azure, or GCP. The resource types include:

* [Cluster](apis/primitives/XCluster/), a resource that loosely represents a Kubernetes cluster.
* [NodePool](apis/primitives/XNodePool/), a resource that loosely represents a Nodepool in a Kubernetes cluster.
* [Database](apis/primitives/XDatabase/), a resource that loosely represents a cloud database.
* [Network](apis/primitives/XNetwork/), a resource that loosely represents a cloud network resource.
* [Subnetwork](apis/primitives/XSubnetwork/), a resource that loosely represents a subnetwork resource within a cloud network.
* [Service Account](apis/primitives/XServiceAccount/), a resource that loosely represents a service account in the cloud.

This configuration also demonstrates the power of Crossplane to build abstractions called "compositions", which assemble multiple basic resources into a more complex resource. These are demonstrated with:

* [CompositeCluster](apis/composition-basics/XCompositeCluster/), a resource abstraction that composes a cluster, nodepool, network, subnetwork, and service account.
* [AccountScaffold](apis/composition-basics/XAccountScaffold/), a resource abstraction that composes a service account, network, and subnetwork.

Learn more about Composite Resources in the [Crossplane
Docs](https://docs.crossplane.io/latest/concepts/compositions/).

## Quickstart

### Prerequisites

Before we can install the reference platform we should install the `up` CLI.
This is a utility that makes following this quickstart guide easier. Everything
described here can also be done in a declarative approach - which we highly
recommend for any production type use-case.
<!-- TODO enhance this guide: Getting ready for Gitops -->

To install `up` run this install script:
```console
curl -sL https://cli.upbound.io | sh
```
See [up docs](https://docs.upbound.io/cli/) for more install options.

We need a running Crossplane control plane to install our instance. Use [Upbound](https://console.upbound.io) to create a managed control plane. You can [create an account](https://accounts.upbound.io/register) and start a free 30 day trial if you haven't signed up for Upbound before.

### Install the Getting Started configuration

Now you can install this reference platform. It's packaged as a [Crossplane
configuration package](https://docs.crossplane.io/latest/concepts/packages/)
so there is a single command to install it:

```console
up ctp configuration install xpkg.upbound.io/upbound/configuration-getting-started:v0.1.0
```

Validate the install by inspecting the provider and configuration packages:
```console
kubectl get providers,providerrevision

kubectl get configurations,configurationrevisions
```

Check the
[marketplace](https://marketplace.upbound.io/configurations/upbound/configuration-getting-started/)
for the latest version of this platform.

## Using the Getting Started configuration

🎉 Congratulations. You have just installed your first Crossplane-powered platform!

You can now use the managed control plane to request resources which will simulate getting provisioned in an external cloud service. You do this by creating "claims" against the APIs available on yuor control palne. In our example here we simply create the claims directly:

Create a custom defined cluster:
```console
kubectl apply -f examples/XCluster/claim.yaml
```

Create a custom defined database:
```console
kubectl apply -f examples/XDatabase/claim.yaml
```

You can verify the status by inspecting the claims, composites and managed
resources:

```console
kubectl get claim,composite,managed
```

To delete the provisioned resources you would simply delete the claims:

```console
kubectl delete -f examples/XCluster/claim.yaml,examples/XDatabase/claim.yaml
```

To uninstall the provider & platform configuration:

```console
kubectl delete configurations.pkg.crossplane.io configuration-getting-started
```

## Next Steps

We recommend you check out of one of Upbound's platform reference architectures to learn how to use Crossplane to provision real external resources, such as in a Cloud Serice Provider's environment. Have a look:

* [AWS reference platform](https://github.com/upbound/platform-ref-aws/)
* [Azure reference platform](https://github.com/upbound/platform-ref-azure/)
* [GCP reference platform](https://github.com/upbound/platform-ref-gcp/)

## Questions?

For any questions, thoughts and comments don't hesitate to [reach
out](https://www.upbound.io/contact) or drop by
[slack.crossplane.io](https://slack.crossplane.io), and say hi!