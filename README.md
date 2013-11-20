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
$ ls services/
web.1.service	web.2.service	web.3.service
$ corectl start services/*
$ corectl list-units
UNIT	LOAD	ACTIVE	SUB	DESC	MACHINE
web.1.service	loaded	inactive	-			My Website  mach2
web.2.service	loaded	inactive	-			My Website  mach2
web.3.service	loaded	active	active		My Website  mach1
web.1.socket	loaded  active 	listening	My Website 	mach2
web.2.socket	loaded  active	listening	My Website 	mach2
web.3.socket	loaded  active  listening  	My Website 	mach1
$ corectl status web.1.service
web.1.service - CoreOS Website
	Loaded: loaded (...)
	Active: active (running)
$ corectl stop web.1.service
```
