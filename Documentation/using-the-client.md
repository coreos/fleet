# Using the Client

coreinit provides a command-line tool called `corectl`. The commands provided by corectl are generally identical to those of systemd's CLI, `systemctl`, while enabling a user to interact with an entire cluster of disconnected systemd instances.

## Get up and running

The `corectl` binary is included in all CoreOS distributions, so it is as simple as SSH'ing in to your CoreOS machine and executing `corectl`.

If you prefer to execute corectl from an external host (i.e. your laptop), the `--tunnel` flag can be used to communicate with an etcd endpoint over an SSH tunnel:

    corectl --tunnel <IP[:PORT]> list-units

Usage of the `--tunnel` flag requires two things:

1. SSH access to the provided address
2. ssh-agent must be running on the client machine with the necessary private key to access the provided address. 

One can SSH directly to the tunnel host to assert ssh-agent is running with the proper access.

corectl also requires direct communication with the etcd cluster that your coreinit machines are configured to use. Use the `--endpoint` flag to override the default of `http://127.0.0.1:4001`:

    corectl --endpoint http://<IP:PORT> list-units

When using the `--tunnel` flag and the `--endpoint` flag together, it is important to note that all etcd requests will be made through the SSH tunnel. The address in the `--endpoint` flag must be routable from the server hosting the tunnel.

## Interact with units

For information about the additional unit file parameters coreinit will interact with, see [this documentation](unit-files.md).

### Explore existing units

List all units in the coreinit cluster with `corectl list-units`. This will describe all units the coreinit cluster knows about, running or not:

```
$ corectl list-units
UNIT			LOAD	ACTIVE	SUB		DESC	MACHINE
hello.service	loaded	active	running	-	148a18ff-6e95-4cd8-92da-c9de9bb90d5a
ping.service	-		-		-		-	-
pong.service	-		-		-		-	-
```

### Push units into a cluster

Getting units into the cluster is as simple as a call to `corectl submit` with a path to one or more unit files:

```
$ corectl submit examples/hello.service
```
You can also rely on your shell's path-expansion to conveniently submit a large set of unit files:

```
$ ls examples/
hello.service	ping.service	pong.service
$ corectl submit examples/*
```

Submission of units to a coreinit cluster does not cause them to be scheduled out to specific hosts. The unit should be visible in a `corectl list-units` command, but have no reported state.

### Start and stop units

Once a unit has been submitted to the coreinit cluster, it can be started and stopped like so:

```
$ corectl start hello.service
```

The `start` operation is what causes a unit to be scheduled to a specific host and executed.

Halting execution of a unit is as simple as calling `stop`:

```
$ corectl stop hello.service
```

### Remove units from a cluster

A unit can be removed from a cluster with the `destroy` command:

```
$ corectl destroy hello.service
```

The `destroy` command does two things:

1. Instruct systemd on the host machine to stop the unit, deferring to systemd completely for any custom stop directives (i.e. ExecStop option in the unit file).
2. Remove the unit file from the cluster, making it impossible to start again until it has been re-submitted.

## Inspect hosts

Describe all of the machines currently connected to the cluster with `corectl list-machines`:

```
$ corectl list-machines
MACHINE									IP			METADATA
148a18ff-6e95-4cd8-92da-c9de9bb90d5a	19.4.0.112	region=us-west
491586a6-508f-4583-a71d-bfc4d146e996	19.4.0.113	region=us-east
```