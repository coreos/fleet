# fleet - a Distributed init System.

fleet ties together [systemd](http://coreos.com/using-coreos/systemd) and [etcd](https://github.com/coreos/etcd) into a distributed init system. Think of it as an extension of systemd that operates at the cluster level instead of the machine level.

This project is very low level and is designed as a foundation for higher order orchestration. This is a preview release – please read the [security notice](Documentation/security.md).

[![Build Status](https://travis-ci.org/coreos/fleet.png?branch=master)](https://travis-ci.org/coreos/fleet)

## Common Uses

fleet allows you to define flexible architectures for running your services:

* Deploy a single container anywhere on the cluster
* Deploy multiple copies of the same container
* Ensure that containers are deployed together on the same machine
* Forbid specific services from co-habitation
* Maintain N containers of a service, re-deploying on failure
* Deploy containers on machines matching specific metadata

## Examples

### List Machines in the Cluster
```
$ fleetctl list-machines
MACHINE									IP			METADATA
148a18ff-6e95-4cd8-92da-c9de9bb90d5a	19.4.0.112	region=us-west
491586a6-508f-4583-a71d-bfc4d146e996	19.4.0.113	region=us-east
```

### Submit & Start Units

```
$ ls examples/
hello.service	ping.service	pong.service
$ fleetctl submit examples/*
$ fleetctl start hello.service
```

### List Units

```
$ fleetctl list-units
UNIT			LOAD	ACTIVE	SUB		DESC	MACHINE
hello.service	loaded	active	running	-		148a18ff-6e95-4cd8-92da-c9de9bb90d5a
ping.service	-		-		-		-		-
pong.service	-		-		-		-		-
```

## Getting Started

Before you can deploy units, fleet must be [deployed][deploy] and [configured][configure] on each host in your cluster. After you have machines configured (`fleetctl list-machines`), [start some units][using-the-client.md].

[using-the-client.md]: https://github.com/coreos/fleet/blob/master/Documentation/using-the-client.md
[deploy]: https://github.com/coreos/fleet/blob/master/Documentation/deployment.md
[configure]: https://github.com/coreos/fleet/blob/master/Documentation/configuration.md

### Building

fleet must be built with Go 1.2 on a Linux machine, or in a [Go docker container](https://index.docker.io/u/miksago/ubuntu-go/). Simply run `scripts/build` and then copy the binaries out of bin/ onto each of your machines.

## Project Details

### APIs

The current fleet interfaces should not be considered stable. Expect incompatible changes in subsequent releases.

### Contributing

See [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches and contacting developers via IRC and mailing lists.

### License

fleet is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
