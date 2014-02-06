# Using the Client

fleet provides a command-line tool called `corectl`. The commands provided by corectl are generally identical to those of systemd's CLI, `systemctl`, while enabling a user to interact with an entire cluster of disconnected systemd instances.

## Get up and running

The `corectl` binary is included in all CoreOS distributions, so it is as simple as SSH'ing in to your CoreOS machine and executing `corectl`.

If you prefer to execute corectl from an external host (i.e. your laptop), the `--tunnel` flag can be used to communicate with an etcd endpoint over an SSH tunnel:

    corectl --tunnel <IP[:PORT]> list-units

Usage of the `--tunnel` flag requires two things:

1. SSH access to the provided address
2. ssh-agent must be running on the client machine with the necessary private key to access the provided address. 

One can SSH directly to the tunnel host to assert ssh-agent is running with the proper access.

corectl also requires direct communication with the etcd cluster that your fleet machines are configured to use. Use the `--endpoint` flag to override the default of `http://127.0.0.1:4001`:

    corectl --endpoint http://<IP:PORT> list-units

When using the `--tunnel` flag and the `--endpoint` flag together, it is important to note that all etcd requests will be made through the SSH tunnel. The address in the `--endpoint` flag must be routable from the server hosting the tunnel.

## Interact with units

For information about the additional unit file parameters fleet will interact with, see [this documentation](unit-files.md).

### Explore existing units

List all units in the fleet cluster with `corectl list-units`. This will describe all units the fleet cluster knows about, running or not:

```
$ corectl list-units
UNIT			LOAD	ACTIVE	SUB		DESC	MACHINE
hello.service	loaded	active	running	-	148a18ff-6e95-4cd8-92da-c9de9bb90d5a
ping.service	-		-		-		-	-
pong.service	-		-		-		-	-
```

### Push units into the cluster

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

Submission of units to a fleet cluster does not cause them to be scheduled out to specific hosts. The unit should be visible in a `corectl list-units` command, but have no reported state.

### Remove units from the cluster

A unit can be removed from a cluster with the `destroy` command:

```
$ corectl destroy hello.service
```

The `destroy` command does two things:

1. Instruct systemd on the host machine to stop the unit, deferring to systemd completely for any custom stop directives (i.e. ExecStop option in the unit file).
2. Remove the unit file from the cluster, making it impossible to start again until it has been re-submitted.

### View unit contents

The contents of a loaded unit file can be printed to stdout using the `corectl cat` command:

```
$ corectl cat examples/hello.service
[Unit]
Description=Hello World

[Service]
ExecStart=/bin/bash -c "while true; do echo \"Hello, world\"; sleep 1; done"
```

### Start and stop units

Once a unit has been submitted to the fleet cluster, it can be started and stopped like so:

```
$ corectl start hello.service
```

The `start` operation is what causes a unit to be scheduled to a specific host and executed.

Halting execution of a unit is as simple as calling `stop`:

```
$ corectl stop hello.service
```

### Query unit status

Once a unit has been started, fleet will publish its status. The systemd state fields 'LoadState', 'ActiveState', and 'SubState' can be retrieved with `corectl list-units`. To get all of the unit's state information, the `corectl status` command will actually call systemctl on the machine running a given unit over SSH:

```
$ corectl status hello.service
hello.service - Hello World
   Loaded: loaded (/run/systemd/system/hello.service; enabled-runtime)
   Active: active (running) since Wed 2014-01-29 23:20:23 UTC; 1h 49min ago
 Main PID: 6973 (bash)
   CGroup: /system.slice/hello.1.service
           ├─ 6973 /bin/bash -c while true; do echo "Hello, world"; sleep 1; done
           └─20381 sleep 1

Jan 30 01:09:18 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:19 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:20 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:21 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:22 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:23 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:24 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:25 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:26 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:09:27 ip-172-31-5-250 bash[6973]: Hello, world
```

### Fetch unit logs

The `corectl journal` command can be used to interact directly with `journalctl` on the machine running a given unit:

```
$ corectl journal hello.service
-- Logs begin at Wed 2014-01-29 20:50:48 UTC, end at Thu 2014-01-30 01:14:55 UTC. --
Jan 30 01:14:46 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:47 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:48 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:49 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:50 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:51 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:52 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:53 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:54 ip-172-31-5-250 bash[6973]: Hello, world
Jan 30 01:14:55 ip-172-31-5-250 bash[6973]: Hello, world
```

## Exploring the cluster

### Enumerate hosts

Describe all of the machines currently connected to the cluster with `corectl list-machines`:

```
$ corectl list-machines
MACHINE									IP			METADATA
148a18ff-6e95-4cd8-92da-c9de9bb90d5a	19.4.0.112	region=us-west
491586a6-508f-4583-a71d-bfc4d146e996	19.4.0.113	region=us-east
```

### SSH dynamically to host

The `corectl ssh` command can be used to open a pseuto-terminal over SSH to a host in the fleet cluster. The command will look up the IP address of a machine based on the provided machine ID:

```
$ corectl ssh 491586a6-508f-4583-a71d-bfc4d146e996
   ______                ____  _____
  / ____/___  ________  / __ \/ ___/
 / /   / __ \/ ___/ _ \/ / / /\__ \
/ /___/ /_/ / /  /  __/ /_/ /___/ /
\____/\____/_/   \___/\____//____/
core@ip-172-31-5-251 ~ $
```

Alternatively, a unit name can be provided using the `--unit` flag. `corectl ssh --unit <UNIT>` will look up the location of the
provided unit in the cluster before opening an SSH connection:

```
$ corectl ssh --unit hello.service
   ______                ____  _____
  / ____/___  ________  / __ \/ ___/
 / /   / __ \/ ___/ _ \/ / / /\__ \
/ /___/ /_/ / /  /  __/ /_/ /___/ /
\____/\____/_/   \___/\____//____/
core@ip-172-31-5-250 ~ $
```
