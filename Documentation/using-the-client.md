# Using the Client

## Running corectl

The Coreinit client utility `corectl` can be run locally on your workstation or from a machine in the cluster.

## List Machines

List out all of the machines currently connected to the cluster:

```
$ corectl list-machines
MACHINE									METADATA
148a18ff-6e95-4cd8-92da-c9de9bb90d5a
491586a6-508f-4583-a71d-bfc4d146e996
```

## Writing Unit Files

Coreinit schedules each unit file to a machine in the cluster, taking into account a few special properties under the `X-Coreinit` section. If you're new to using systemd unit files, check out the [Getting Started with systemd](https://coreos.com/docs/launching-containers/launching/getting-started-with-systemd) guide.

| Property Name | Description |
|---------------|-------------|
| X-Coreinit-Provides | A string that represents the service name. This is used to enforce `X-Coreinit-MachineSingleton`. |
| X-Coreinit-Peers | The name of a unit file that should be scheduled alongside this unit. |
| X-Coreinit-MachineSingleton | Boolean value that controls whether multiple copies of this service can run on a single machine. Useful for creating HA services. |

To deploy multiple copies of a service, generate unique copies of each unit file. Each will need its own name (`apache1.service`) and contents where appropriate. In the following example, the name of the container `apache1` is unique. These unit files should be considered perfectly cachable. Any changes will need to result in a new unit file.

```
[Unit]
Description=Example service started with coreinit
After=docker.service

[Service]
ExecStart=/usr/bin/docker run -name apache1 coreos/apache /usr/sbin/apache2ctl -D FOREGROUND
ExecStop=/usr/bin/docker stop apache1

[X-Coreinit]
X-Coreinit-Provides=apache
X-Coreinit-MachineSingleton=true
```

Other Notes
* Unit files must not have an [Install] section
* Unit file names must end in '.service' or '.socket'

## Running Unit Files

Running a unit on the cluster is done by specifying a unit file — just as if you were using systemd locally — but you're operating at the cluster level instead of the machine level. An included tool, `corectl` acts similarly to `systemctl`. To run the unit files you just generated, run:

```
corectl start apache1.service
```

You can also start an entire folder:

```
$ ls examples/
socket-activated.service  socket-activated.socket  web@8000.service  web@8001.service  web@8002.service
$ corectl start examples/*
```

## List Units

You can list all of the units cluster wide and get a quick status:

```
$ corectl list-units
UNIT						LOAD	ACTIVE		SUB			DESC							MACHINE
socket-activated.service	loaded	inactive	dead        Socket-Activated Web Service	491586a6-508f-4583-a71d-bfc4d146e996
socket-activated.socket		loaded	active		listening 	Socket-Activated Web Service	491586a6-508f-4583-a71d-bfc4d146e996
web@8000.service			loaded	active		running		Web Service						491586a6-508f-4583-a71d-bfc4d146e996
web@8001.service			loaded	active		running		Web Service						148a18ff-6e95-4cd8-92da-c9de9bb90d5a
web@8002.service			loaded	active		running		Web Service						491586a6-508f-4583-a71d-bfc4d146e996
apache1.service				loaded	active		running		Example service started with c	148a18ff-6e95-4cd8-92da-c9de9bb90d5a
```