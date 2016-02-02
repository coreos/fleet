# Using the Client

fleet provides a command-line tool called `fleetctl`. The commands provided by
`fleetctl` are analogous to those of systemd's CLI, `systemctl`.

## Get up and running

The `fleetctl` binary is included in all CoreOS distributions, so it is as simple as SSH'ing in to your CoreOS machine and executing `fleetctl`.

### Custom API Endpoint

`fleetctl` communicates directly with an HTTP API hosted by the fleet cluster. Use the `--endpoint` flag to override the default of `unix:///var/run/fleet.sock`:

```sh
fleetctl --endpoint http://<IP:PORT> list-units
```

Alternatively, `--endpoint` can be provided through the `FLEETCTL_ENDPOINT` environment variable:

```sh
FLEETCTL_ENDPOINT=http://<IP:[PORT]> fleetctl list-units
```

*It is not recommended to listen fleet API TCP socket over public and even private networks.* Fleet API socket doesn't support encryption and authorization so it could cause full root access to your machine. Please use [ssh tunnel][ssh-tunnel] to access remote fleet API.

### From an External Host

If you prefer to execute fleetctl from an external host (i.e. your laptop), the `--tunnel` flag can be used to tunnel communication with your fleet cluster over SSH:

```sh
fleetctl --tunnel <IP[:PORT]> list-units
```

One can also provide `--tunnel` through the environment variable `FLEETCTL_TUNNEL`:

```sh
FLEETCTL_TUNNEL=<IP[:PORT]> fleetctl list-units
```

When using `--tunnel` and `--endpoint` together, it is important to note that all etcd requests will be made through the SSH tunnel. 
The address in the `--endpoint` flag must be routable from the server hosting the tunnel.

If the external host requires a username other than `core`, the `--ssh-username` flag can be used to set an alternative username.

```sh
fleetctl --ssh-username=elroy list-units
````

Or

```sh
FLEETCTL_SSH_USERNAME=elroy fleetctl list-units
```

Note: Custom users are not by default part of the `systemd-journal` group which will cause you to see `No journal files were found.`
To use the `journal` command please add your users to the `systemd-journal` group or use the `--sudo` flag with journal.

Be sure to install one of the [tagged releases](https://github.com/coreos/fleet/releases) of `fleetctl` that matches the version of fleet running on the CoreOS machine.
Find the version on the server with:

```sh
fleet --version
```

See more about [configuring remote access](#remote-fleet-access).

## Interacting with units

For information regarding the additional unit file parameters that modify fleet's behavior, see [this documentation](https://github.com/coreos/fleet/blob/master/Documentation/unit-files-and-scheduling.md).

### Explore existing units

List all units in the fleet cluster with `fleetctl list-unit-files`:

```sh
$ fleetctl list-unit-files
UNIT            HASH    DSTATE   STATE    TMACHINE
goodbye.service d4c61bf launched launched 85c0c595.../172.17.8.102
hello.service   e55c0ae launched launched 113f16a7.../172.17.8.103
```

`fleetctl list-unit-files` communicates what the desired state of a unit is, what its current state is, and where it is currently scheduled.

List the last-known state of fleet's active units (i.e. those loaded onto a machine) with `fleetctl list-units`:

```sh
$ fleetctl list-units
UNIT            MACHINE                   ACTIVE  SUB
goodbye.service 85c0c595.../172.17.8.102  active  running
hello.service   113f16a7.../172.17.8.103  active  running
```

### Start and stop units

Start and stop units with the `start` and `stop` commands:

```sh
$ fleetctl start goodbye.service
Unit goodbye.service launched on 85c0c595.../172.17.8.102

$ fleetctl stop goodbye.service
Unit goodbye.service loaded on 85c0c595.../172.17.8.102
```

If the unit does not exist when calling `start`, fleetctl will first search for a local unit file, submit it and schedule it.

### Scheduling units

To schedule a unit into the cluster (i.e. load it on a machine) without starting it, call `fleetctl load`:

```sh
$ fleetctl load hello.service
Unit hello.service loaded on 113f16a7.../172.17.8.103
```

This will not call the equivalent of `systemctl start`, so the loaded unit will be in an inactive state:

```sh
$ fleetctl list-units
UNIT          MACHINE                  ACTIVE   SUB
hello.service 113f16a7.../172.17.8.103 inactive dead
```

This is useful if you have another unit that will activate it at a later date, such as a path or timer.

Units can also be unscheduled, but remain in the cluster with `fleetctl unload`.
The unit will still be visible in `fleetctl list-unit-files`, but will have no state reported in `fleetctl list-units`:

```sh
$ fleetctl unload hello.service

$ fleetctl list-unit-files
UNIT          HASH    DSTATE   STATE    TMACHINE
hello.service e55c0ae inactive inactive -
```

### Adding and removing units

Getting units into the cluster is as simple as a call to `fleetctl submit`:

```sh
$ fleetctl submit examples/hello.service
```

You can also rely on your shell's path-expansion to conveniently submit a large set of unit files:

```sh
$ ls examples/
hello.service	ping.service	pong.service
$ fleetctl submit examples/*
```

Submission of units to a fleet cluster does not cause them to be scheduled. 
The unit will be visible in a `fleetctl list-unit-files` command, but have no reported state in `fleetctl list-units`.

A unit can be removed from a cluster with the `destroy` command:

```sh
$ fleetctl destroy hello.service
```

The `destroy` command does two things:

1. Instruct systemd on the host machine to stop the unit, deferring to systemd completely for any custom stop directives (i.e. `ExecStop` option in the unit file).
2. Remove the unit file from the cluster, making it impossible to start again until it has been re-submitted.

Once a unit is destroyed, state will continue to be reported for it in `fleetctl list-units`.
Only once the unit has stopped will its state be removed.

### View unit contents

The contents of a loaded unit file can be printed to stdout using the `fleetctl cat` command:

```sh
$ fleetctl cat hello.service
[Unit]
Description=Hello World

[Service]
ExecStart=/bin/bash -c "while true; do echo \"Hello, world\"; sleep 1; done"
```

### Query unit status

Once a unit has been started, fleet will publish its status. The systemd state fields 'LoadState', 'ActiveState', and 'SubState' can be retrieved with `fleetctl list-units`. To get all of the unit's state information, the `fleetctl status` command will actually call systemctl on the machine running a given unit over SSH:

```sh
$ fleetctl status hello.service
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

The `fleetctl journal` command can be used to interact directly with `journalctl` on the machine running a given unit:

```sh
$ fleetctl journal hello.service
-- Logs begin at Thu 2014-08-21 18:27:04 UTC, end at Thu 2014-08-21 19:07:38 UTC. --
Aug 21 19:07:38 core-03 bash[1127]: Hello, world
```

## Exploring the cluster

### Enumerate hosts

Describe all of the machines currently connected to the cluster with `fleetctl list-machines`:

```sh
$ fleetctl list-machines
MACHINE     IP           METADATA
113f16a7... 172.17.8.103 az=us-west-1b
85c0c595... 172.17.8.102 az=us-west-1b
e793afb9... 172.17.8.101 az=us-west-1a
```

### SSH dynamically to host

The `fleetctl ssh` command can be used to open a pseudo-terminal over SSH to a host in the fleet cluster.
The command will look up the IP address of a machine based on the provided machine ID:

```sh
$ fleetctl ssh 113f16a7
```

Alternatively, a unit name can be provided.
`fleetctl ssh` will connect to the machine to-which the given unit is scheduled:

```sh
$ fleetctl ssh hello.service
```

### Known-Hosts Verification

Fingerprints of machines accessed through fleetctl are stored in `$HOME/.fleetctl/known_hosts` and used for the verification of machine identity.
If a machine presents a fingerprint that differs from that found in the known_hosts file, the SSH connection will be aborted.

Disable the storage of fingerprints with `--strict-host-key-checking=false`, or change the location of your fingerprints with the `--known-hosts-file=<LOCATION>` flag.


# Remote fleet Access

fleet does not yet have any custom authentication, so security of a given fleet cluster depends on a user's ability to access any host in that cluster. The suggested method of authentication is public SSH keys and ssh-agents. The `fleetctl` command-line tool can assist in this by natively interacting with an ssh-agent to authenticate itself.

This requires two things:

1. SSH access for a user to at least one host in the cluster
2. ssh-agent running on a user's machine with the necessary identity

Authorizing a user's SSH key within a cluster is up to the deployer. See the [deployment doc][d] for help doing this.

Running an ssh-agent is the responsibility of the user. Many unix-based distros conveniently provide the necessary tools on a base install, or in an ssh-related package. For example, Ubuntu provides the `ssh-agent` and `ssh-add` binaries in the `openssh-client` package. If you cannot find the necessary binaries on your system, please consult your distro's documentation.

Assuming you have the tools installed, simply ensure ssh-agent has the necessary identity:

```sh
$ ssh-add ~/.ssh/id_rsa
Identity added: id_rsa (~/.ssh/id_rsa)
$ ssh-add -l
2048 31:c3:50:2b:44:f9:7f:28:6b:62:96:37:c7:c1:b5:c2 id_rsa (RSA)
```

To verify the ssh-agent and remote hosts are properly configured, simply connect directly to a host in the fleet cluster using `ssh`. Configure `fleetctl` to tunnel through that host by setting the `--tunnel` flag or exporting the `FLEETCTL_TUNNEL` environment variable:

```sh
$ fleetctl --tunnel 192.0.2.14:2222 list-units
...
$ FLEETCTL_TUNNEL=192.0.2.14:2222 fleetctl list-units
...
```

## Vagrant

Things get a bit more complicated when using [vagrant][v], as access to your hosts is abstracted away from the user. This makes it a bit more complicated to run `fleetctl` from your local laptop, but it's still relatively easy to configure.

First, find the identity file used by vagrant to authenticate access to your hosts. The `vagrant` binary provides a convenient `ssh-config` command to help do this. Running `vagrant ssh-config` from a Vagrant project directory will produce something like this:

```
% vagrant ssh-config
Host default
  HostName 127.0.0.1
  User core
  Port 2222
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  IdentityFile /Users/bcwaldon/.vagrant.d/insecure_private_key
  IdentitiesOnly yes
  LogLevel FATAL
  ForwardAgent yes
```

The output communicates exactly how the connection to a vagrant host is made when calling `vagrant ssh`. Using the `HostName`, `Port` and `IdentityFile` options, we can bypass `vagrant ssh` and connect directly:

```sh
$ ssh -p 2222 -i /Users/bcwaldon/.vagrant.d/insecure_private_key core@127.0.0.1
Last login: Thu Feb 20 05:39:51 UTC 2014 from 10.0.2.2 on pts/1
CoreOS (alpha)
core@localhost ~ $
```

Now, let's get `fleetctl` working with these parameters:

```sh
$ vagrant ssh-config | sed -n "s/IdentityFile//gp" | xargs ssh-add
Identity added: /Users/bcwaldon/.vagrant.d/insecure_private_key (/Users/bcwaldon/.vagrant.d/insecure_private_key)
$ export FLEETCTL_TUNNEL="$(vagrant ssh-config | sed -n "s/[ ]*HostName[ ]*//gp"):$(vagrant ssh-config | sed -n "s/[ ]*Port[ ]*//gp")"
$ echo $FLEETCTL_TUNNEL
127.0.0.1:2222
$ fleetctl list-machines
...
```

The `ssh-add` command need only be run once for all Vagrant hosts. You will have to set `FLEETCTL_TUNNEL` specifically for each vagrant host with which you interact.

[v]: http://www.vagrantup.com/
[ssh-tunnel]: #from-an-external-host
[d]: deployment-and-configuration.md
