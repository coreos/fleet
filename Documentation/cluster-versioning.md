# Cluster Versioning

In order to support no-downtime rolling upgrades within fleet, a given fleet cluster must agree on how to communicate with its shared datastore.
fleet accomplishes this by making a shared decision about the current version, taking into account the supportable versions of all known clients. 

## Negotiator

Every client of the Registry must be represented by a Negotiator.
It is the responsibility of a Negotiator to publish its supportable versions to the greater cluster.
A given Negotiator's supportable versions are represented as an inclusive range of two positive integers.
For example, a Negotiator could publish a minimum supported version of 3 and a maximum of 10.

## Joining a Cluster

Joining a cluster is a three-step process:

- acquire cluster version lock
- assert the cluster version is within the Negotiator's supportable range
- release cluster version lock

This prevents a Negotiator from joining a cluster where it can not possibly play well with others.

### Boostrapping a New Cluster

If a Negotiator is the first to join a cluster, it must act slightly differently:

- acquire cluster version lock
- set the cluster version to the maximum supported by the Negotiator
- release cluster version lock

## Leaving a Cluster

Leaving a cluster can be done without the use of a lock.
All a Negotiator must do is remove their published state from the cluster.

## Upgrading a Cluster

As Negotiators join and leave a cluster, the cluster-level supportable version range will change as well.
If a given Negotiator ever sees that an upgrade of the cluster version is possible, it may do so:

- acquire cluster version lock
- set the cluster version to an integer within the shared supportable range
- release cluster version lock

The new cluster version must be greater than the current version, while remaining within the supportable range of all known Negotiators.