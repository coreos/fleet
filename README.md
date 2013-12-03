## coreinit - a distributed init system.

coreinit ties together systemd and etcd into a distributed init system.

### Assumptions

Machines have truly unique UUIDs and their metadata is perfectly cacheable.
If a machine changes IP addresses, etc it *must* have a new UUID.

#### Unit Files
* Unit files must not have an [Install] section
* Unit file names must end in '.service' or '.socket'

### Using corectl
```
$ ls examples/
socket-activated.service  socket-activated.socket  web@8000.service  web@8001.service  web@8002.service
$ corectl start examples/*
$ corectl list-units
UNIT						LOAD	ACTIVE		SUB			DESC							MACHINE
socket-activated.service	loaded	inactive	dead        Socket-Activated Web Service	491586a6-508f-4583-a71d-bfc4d146e996
socket-activated.socket		loaded	active		listening 	Socket-Activated Web Service	491586a6-508f-4583-a71d-bfc4d146e996
web@8000.service			loaded	active		running		Web Service						491586a6-508f-4583-a71d-bfc4d146e996
web@8001.service			loaded	active		running		Web Service						491586a6-508f-4583-a71d-bfc4d146e996
web@8002.service			loaded	active		running		Web Service						491586a6-508f-4583-a71d-bfc4d146e996
$ corectl status web@8000.service
web@8000.service - Web Service
	Loaded: loaded
	Active: active (running)

$ corectl stop web@8000.service
```
