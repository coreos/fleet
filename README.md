# fleet - a distributed init system

[![Build Status](https://travis-ci.org/coreos/fleet.png?branch=master)](https://travis-ci.org/coreos/fleet)
[![Build Status](https://semaphoreci.com/api/v1/coreos/fleet/branches/master/badge.svg)](https://semaphoreci.com/coreos/fleet)

fleet ties together [systemd](http://coreos.com/using-coreos/systemd) and [etcd](https://github.com/coreos/etcd) into a simple distributed init system. Think of it as an extension of systemd that operates at the cluster level instead of the machine level.

**This project is very low level and is designed as a foundation for higher order orchestration.** fleet is oriented around systemd units and is not a container manager or orchestration system. fleet supports very basic scheduling of systemd units in a cluster. Those looking for more complex scheduling requirements or a first-class container orchestration system should check out [Kubernetes](http://kubernetes.io).

## Current status

fleet has seen production use for some time and is largely considered stable. 
However, there are known [scalability limitations](Documentation/fleet-scaling.md) with its architecture.
As such, it is not recommended to run fleet clusters larger than 100 nodes or with more than 1000 services.
The fleet project is being maintained for bug fixes but the existing maintainers do not intend to add additional major features or significantly rework fleet to address these limitations.

If you are a developer and wish to help maintain fleet and improve its scalability, please email the [maintainers](MAINTAINERS).

## Using fleet

Launching a unit with fleet is as simple as running `fleetctl start`:

```
$ fleetctl start examples/hello.service
Unit hello.service launched on 113f16a7.../172.17.8.103
```

The `fleetctl start` command waits for the unit to get scheduled and actually start somewhere in the cluster.
`fleetctl list-unit-files` tells you the desired state of your units and where they are currently scheduled:

```
$ fleetctl list-unit-files
UNIT            HASH     DSTATE    STATE     TMACHINE
hello.service   e55c0ae  launched  launched  113f16a7.../172.17.8.103
```

`fleetctl list-units` exposes the systemd state for each unit in your fleet cluster:

```
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

[unit-files]: https://github.com/coreos/fleet/blob/master/Documentation/unit-files-and-scheduling.md#fleet-specific-options

## Getting Started

Before you can deploy units, fleet must be [deployed and configured][deploy-and-configure] on each host in your cluster. (If you are running CoreOS, fleet is already installed.)

After you have machines configured (check `fleetctl list-machines`), get to work with the [client][using-the-client.md].

[using-the-client.md]: https://github.com/coreos/fleet/blob/master/Documentation/using-the-client.md
[deploy-and-configure]: https://github.com/coreos/fleet/blob/master/Documentation/deployment-and-configuration.md

### Building

fleet must be built with Go 1.4+ on a Linux machine. Simply run `./build` and then copy the binaries out of bin/ onto each of your machines. The tests can similarly be run by simply invoking `./test`.

If you're on a machine without Go 1.4+ but you have Docker installed, run `./build-docker` to compile the binaries instead.

## Project Details

### API

The fleet API uses JSON over HTTP to manage units in a fleet cluster.
See the [API documentation][api-doc] for more information.

[api-doc]: https://github.com/coreos/fleet/blob/master/Documentation/api-v1.md

### Release Notes

See the [releases tab](https://github.com/coreos/fleet/releases) for more information on each release.

### Contributing

See [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches and contacting developers via IRC and mailing lists.

### License

fleet is released under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.

Specific components of fleet use code derivative from software distributed under other licenses; in those cases the appropriate licenses are stipulated alongside the code.
