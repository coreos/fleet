# fleet and scaling

As fleet currently uses etcd for cluster wide coordination, making it scale
requires minimizing the load it puts on etcd. This is true for reads, writes,
and watches.

## Known issues

- Currently when fleet schedules a job *all* `fleetd`s are woken up (via a watch)
and then do a recursive GET on the Unit file in etcd to figure out if it should
schedule a job. This is a very expensive operation.

- With a large number of units (~500+), the `fleetd` agent exhibits significant
CPU usage from parsing all D-Bus messages when interacting with systemd.

- With a large number of units, `fleetd` exhibits significant CPU usage when
  - a) parsing the JSON-encoded representations (stored in etcd)
  - b) parsing the unit file itself (using go-systemd)

- The agent deals very poorly with inconsistent read/write latencies with etcdal
  (*what is the actual behaviour?*)

## Improvements

To make fleet scale further in the future we could consider rearchitecting
fleet to provide per-machine schedules (so that only those `fleetd`s are woken
up that actually have work for them). This is akin to the [Thundering herd
problem](https://en.wikipedia.org/wiki/Thundering_herd_problem), but in a
distributed fashion. Once such a change is in we can also drop the periodic
wakeups (agent TTLs) that cause fleet wide wake-ups on a regular clock.

Ultimately, fleet should move away from using etcd as an RPC mechanism.
Instead, it should use etcd only for leader election and then perform direct
RPCs between the engine and agent.

## Implemented quick wins

* Disallowing (some) nodes to partake in the fleet leadership election. Again
    this is an expensive operation. The fewer nodes that are engaged in this
    election, the better. Possible downside is that if there isn't a leader at
    all, the cluster is inoperable. However the (usually) 5 machines running
    etcd are also a single point of failure. *See the `disable_engine` config flag.*

* Making some defaults exported and allow them to be overridden. For instance
    fleet's tokenLimit controls how many Units are listed per "page". *See the
    `token_limit` config flag.*

* Removing watches from fleet: By removing the watches from fleet we stop
    the entire cluster from walking up whenever a new job is to be scheduled.
    The downside of this change is that fleet's responsiveness is lower.
    *See the `disable_watches` config flag.*
