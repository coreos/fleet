# Architecture

## fleetd

Every system in the fleet cluster runs a single `fleetd` daemon. Each daemon encapsulates two roles: the *engine* and the *agent*. An engine primarily makes scheduling decisions while an agent executes units. Both the engine and agent use the _reconciliation model_, periodically generating a snapshot of "current state" and "desired state" and doing the necessary work to mutate the former towards the latter.

### Engine

- The engine is responsible for making scheduling decisions in the cluster. This happens in a reconciliation loop, triggered periodically or by certain events from etcd
- At the start of the reconciliation process, the engine gathers a snapshot of the overall state of the cluster. This includes the set of units in the cluster (and their desired and known states) and the set of agents running in the cluster. The engine then attempts to reconcile the actual state with the desired state
- The engine uses a _lease model_ to enforce that only one engine is running at a time. Every time a reconciliation is due, an engine will attempt to take a lease on etcd. If the lease succeeds, the reconciliation proceeds; otherwise, that engine will remain idle until the next reconciliation period begins.
- The engine uses a simplistic "least-loaded" scheduling algorithm: when considering where to schedule a given unit, preference is given to agents running the smallest number of units.

The reconciliation loop of the engine can be disabled with the `--disable-engine` flag. This means that
this `fleetd` daemon will *never* become a cluster leader. If all running daemons have this setting,
your cluster is dead; i.e. no jobs will be scheduled. Use with care.

### Agent

- The agent is responsible for actually executing Units on systems. It communicates with the local systemd instance over D-Bus.
- Similar to the engine, the agent runs a reconciliation loop which periodically collects a snapshot from etcd to determine what it should be doing. The agent then performs the necessary actions (e.g. loading and starting units) to ensure its "current state" matches its "desired state".
- The agent is also responsible for reporting the state of units to etcd.

## etcd

etcd is the sole datastore in a fleet cluster. All persistent and ephemeral data is stored in etcd: unit files, cluster presence, unit state, etc.

etcd is also used for all internal communication between fleet engines and agents.

## Object Model

### User-facing Objects

#### Units

A Unit represents a single systemd unit file. Once a Unit is pushed to the cluster, its name and underlying contents are immutable; the only flag which can be changed is its desired state. A Unit must be destroyed and re-submitted for any other modifications to be made.

The Unit may define a set of requirements that must be fulfilled by a given host in order for that host to run the Unit. These requirements can include resources, host metadata, locality relative to other Units, etc.

All Units are treated as services rather than batch processes: if a machine on which a Unit is running goes away, fleet will reschedule the Unit elsewhere.

#### State

Both Units and Machines have dynamic state which is published both for the user and cluster to consume.

A UnitState object represents the state of a Unit in the fleet engine. A UnitState object represents the state of a payload as reported by systemd on a given Machine. For more information on states, see the [states documentation].


# Security

## Preview Release

Current releases of fleet don't currently perform any authentication or authorization for submitted units. This means that any client that can access your etcd cluster can potentially run arbitrary code on many of your machines very easily.

## Securing etcd

You should avoid public access to etcd and instead run fleet [from your local laptop][using-the-client] with the `--tunnel` flag to run commands over an SSH tunnel. You can alias this flag for easier usage: `alias fleetctl=fleetctl --tunnel 10.10.10.10` - or use the environment variable `FLEETCTL_TUNNEL`.

## Other Notes

Since it interacts directly with systemd over D-Bus, the fleetd daemon must be run with elevated privileges (i.e. as root) in order to perform operations like starting and stopping services. From the [systemd D-Bus documentation][systemd-dbus]:

> In contrast to most of the other services of the systemd suite PID 1 does not use PolicyKit for controlling access to privileged operations, but relies exclusively on the low-level D-Bus policy language. (This is done in order to avoid a cyclic dependency between PolicyKit and systemd/PID 1.) This means that sensitive operations exposed by PID 1 on the bus are generally not available to unprivileged processes directly.

[states documentation]: states.md
[using-the-client]: using-the-client.md#get-up-and-running
[systemd-dbus]: http://www.freedesktop.org/wiki/Software/systemd/dbus/
