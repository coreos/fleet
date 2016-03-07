# fleet - a distributed init system

[![Build Status](https://travis-ci.org/coreos/fleet.png?branch=master)](https://travis-ci.org/coreos/fleet)
[![Build Status](https://semaphoreci.com/api/v1/coreos/fleet/branches/master/badge.svg)](https://semaphoreci.com/coreos/fleet)

fleet ties together [systemd][coreos-systemd] and [etcd][etcd] into a simple distributed init system. Think of it as an extension of systemd that operates at the cluster level instead of the machine level.

**This project is very low level and is designed as a foundation for higher order orchestration.** fleet is oriented around systemd units and is not a container manager or orchestration system. fleet supports very basic scheduling of systemd units in a cluster. Those looking for more complex scheduling requirements or a first-class container orchestration system should check out [Kubernetes][kubernetes]. [fleet vs kubernetes][fleet-vs-k8s] comparison table should help you to make a right decision.

## Current status

fleet has seen production use for some time and is largely considered stable. 
However, there are known [scalability limitations][fleet-scaling] with its architecture.
As such, it is not recommended to run fleet clusters larger than 100 nodes or with more than 1000 services.
The fleet project is being maintained for bug fixes but the existing maintainers do not intend to add additional major features or significantly rework fleet to address these limitations.

If you are a developer and wish to help maintain fleet and improve its scalability, please email the [maintainers][maintainers].

## Using fleet

Launching a unit with fleet is as simple as running `fleetctl start`:

```sh
$ fleetctl start examples/hello.service
Unit hello.service launched on 113f16a7.../172.17.8.103
```

The `fleetctl start` command waits for the unit to get scheduled and actually start somewhere in the cluster.
`fleetctl list-unit-files` tells you the desired state of your units and where they are currently scheduled:

```sh
$ fleetctl list-unit-files
UNIT            HASH     DSTATE    STATE     TMACHINE
hello.service   e55c0ae  launched  launched  113f16a7.../172.17.8.103
```

`fleetctl list-units` exposes the systemd state for each unit in your fleet cluster:

```sh
$ fleetctl list-units
UNIT            MACHINE                    ACTIVE   SUB
hello.service   113f16a7.../172.17.8.103   active   running
```

## Supported Deployment Patterns

fleet is not intended to be an all-purpose orchestration system, and as such supports only a few simple deployment patterns:

* Deploy a single unit anywhere on the cluster
* Deploy a unit globally everywhere in the cluster
* Automatic rescheduling of units on machine failure
* Ensure that units are deployed together on the same machine
* Forbid specific units from colocation on the same machine (anti-affinity)
* Deploy units to machines only with specific metadata

These patterns are all defined using [custom systemd unit options][unit-files].

## Getting Started

Before you can deploy units, fleet must be [deployed and configured][deploy-and-configure] on each host in your cluster. (If you are running CoreOS, fleet is already installed.)

After you have machines configured (check `fleetctl list-machines`), get to work with the [client][using-the-client.md].

### Building

fleet must be built with Go 1.4+ on a Linux machine. Simply run `./build` and then copy the binaries out of `bin/` directory onto each of your machines. The tests can similarly be run by simply invoking `./test`.

If you're on a machine without Go 1.4+ but you have Docker installed, run `./build-docker` to compile the binaries instead.

## Project Details

### API

The fleet API uses JSON over HTTP to manage units in a fleet cluster.
See the [API documentation][api-doc] for more information.

### Release Notes

See the [releases tab][releases] for more information on each release.

### Contributing

See [CONTRIBUTING][contributing] for details on submitting patches and contacting developers via IRC and mailing lists.

### License

fleet is released under the Apache 2.0 license. See the [LICENSE][license] file for details.

Specific components of fleet use code derivative from software distributed under other licenses; in those cases the appropriate licenses are stipulated alongside the code.

[api-doc]: Documentation/api-v1.md
[contributing]: CONTRIBUTING.md
[coreos-systemd]: https://github.com/coreos/docs/blob/master/os/getting-started-with-systemd.md
[deploy-and-configure]: Documentation/deployment-and-configuration.md
[etcd]: https://github.com/coreos/etcd
[fleet-scaling]: Documentation/fleet-scaling.md
[fleet-vs-k8s]: Documentation/fleet_vs_kubernetes.md
[kubernetes]: http://kubernetes.io
[license]: LICENSE
[maintainers]: MAINTAINERS
[releases]: https://github.com/coreos/fleet/releases
[unit-files]: Documentation/unit-files-and-scheduling.md#fleet-specific-options
[using-the-client.md]: Documentation/using-the-client.md
