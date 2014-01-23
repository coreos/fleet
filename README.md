## coreinit - a distributed init system.

coreinit ties together [systemd](http://coreos.com/using-coreos/systemd) and [etcd](https://github.com/coreos/etcd) into a distributed init system. Think of it as an extension of systemd that operates at the cluster level instead of the machine level.

This project is very low level and is designed as a foundation for higher order orchestration.

[![Build Status](https://travis-ci.org/coreos/coreinit.png?branch=master)](https://travis-ci.org/coreos/coreinit)

### Building

Coreinit must be built with Go 1.2 on a Linux machine, or in a [Go docker container](https://index.docker.io/u/miksago/ubuntu-go/). Simply run `./build` and then copy the binaries onto each of your machines.

### Set Up

coreinit needs to be running on each machine that's part of the cluster. On each CoreOS machine, place the `coreinit` and `corectl` binaries in `/home/core`, the unit into `/media/state/units/coreinit.service`, and the config file into `/home/core/coreinit.conf`.

```
$ cat /media/state/units/coreinit.service
[Unit]
Description=coreinit

[Service]
ExecStart=/home/core/coreinit -config_file /home/core/coreinit.conf
```

You can configure coreinit with a config file. See the example file for more info.

To start your coreinit unit, run `sudo systemctl start coreinit.service`.

### Writing Unit Files

Coreinit schedules each unit file to a machine in the cluster, taking into account a few special properties under the `X-Coreinit` section. 

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


### Running Unit Files

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

### List Units

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

### List Machines

List out all of the machines currently connected to the cluster:

```
$ corectl list-machines
MACHINE									METADATA
148a18ff-6e95-4cd8-92da-c9de9bb90d5a
491586a6-508f-4583-a71d-bfc4d146e996
```

### Assumptions

* Machines have truly unique UUIDs and their metadata is perfectly cacheable.
* If a machine changes IP addresses, etc it *must* have a new UUID.
