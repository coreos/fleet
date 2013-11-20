## coreinit - a distributed init system.

coreinit ties together systemd and etcd into a distributed init system.

### Assumptions

Machines have truly unique UUIDs and their metadata is perfectly cacheable.
If a machine changes IP addresses, etc it *must* have a new UUID.

#### Service Files
* Service files must not have an [Install] section
* Service file names must end in '.service'

### (WIP) Using corectl
```
$ ls examples/
web@8000.service	web@8001.service	web@8002.service
$ corectl start examples/*
$ corectl list-units
UNIT	LOAD	ACTIVE	SUB	DESC	MACHINE
web@8000.service	loaded	active	-	Web Service	491586a6-508f-4583-a71d-bfc4d146e996
web@8001.service	loaded	active	-	Web Service	491586a6-508f-4583-a71d-bfc4d146e996
web@8002.service	loaded	active	-	Web Service	491586a6-508f-4583-a71d-bfc4d146e996
$ corectl status web@8000.service
web@8000.service - Web Service
	Loaded: loaded (...)
	Active: active (running)
$ corectl stop web@8000.service
```
