## coreinit - a distributed init system.

coreinit ties together [systemd](http://coreos.com/using-coreos/systemd) and [etcd](https://github.com/coreos/etcd) into a distributed init system. Think of it as an extension of systemd that operates at the cluster level instead of the machine level.

This project is very low level and is designed as a foundation for higher order orchestration.

[![Build Status](https://travis-ci.org/coreos/coreinit.png?branch=master)](https://travis-ci.org/coreos/coreinit)

### Common Uses

Coreinit allows you to define flexible architectures for running your services:

* Deploy a single container anywhere on the cluster
* Deploy multiple copies of the same container
* Ensure that containers are deployed together on the same machine
* Forbid specific services from co-habitation
* Maintain N containers of a service, re-deploying on failure
* Deploy containers on machines matching specific metadata

### Building

Coreinit must be built with Go 1.2 on a Linux machine, or in a [Go docker container](https://index.docker.io/u/miksago/ubuntu-go/). Simply run `./build` and then copy the binaries onto each of your machines.

### Get Started

Before you can deploy units, coreinit must be [deployed][deploy] and [configured][configure] on each host cluster your cluster. After you have machines configured (`corectl list-machines`), [start some units][using-the-client.md].

[using-the-client.md]: https://github.com/coreos/coreinit/blob/master/Documentation/using-the-client.md
[deploy]: https://github.com/coreos/coreinit/blob/master/Documentation/deployment.md
[configure]: https://github.com/coreos/coreinit/blob/master/Documentation/configuration.md
