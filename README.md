## coreinit - a distributed init system.

coreinit ties together systemd and etcd into a distributed init system.

### Example

```
./build
etcd -f -v
systemctl enable --runtime `pwd`/examples/*
systemctl start simplehttp.service
./coreinit
curl http://127.0.0.1:4001/v1/keys/coreos.com/coreinit/system/simplehttp.service/
```

### Assumptions

Machines have truly unique UUIDs and their metadata is perfectly cacheable.
If a machine changes IP addresses, etc it *must* have a new UUID.

All services that you want to publish are WantedBy a target called
local.target.

### (WIP) Using corectl

```
$ mkdir services/
$ cd services/
$ touch web.service  # this is a systemd service file

$ corectl start --worker=3 web.service
$ corectl list-units
UNIT			LOAD	ACTIVE		SUB			DESC       		MACHINE
web.service.1	loaded	inactive	-			CoreOS Website  mach2
web.service.2	loaded	inactive	-			CoreOS Website  mach2
web.service.3	loaded	active		active		CoreOS Website  mach1
web.socket.1	loaded  active  	listening	CoreOS Website 	mach2
web.socket.2	loaded  active  	listening  	CoreOS Website 	mach2
web.socket.3	loaded  active  	listening  	CoreOS Website 	mach1
$ corectl status web.service.1
web.service.1 - CoreOS Website
	Loaded: loaded (...)
	Active: active (running)
$ corectl status web.service
web.service.1 - CoreOS Website
	Loaded: loaded (...)
	Active: inactive (-)
web.service.2 - CoreOS Website
	Loaded: loaded (...)
	Active: inactive (-)
web.service.3 - CoreOS Website
	Loaded: loaded (...)
	Active: active (running) since XXX

$ corectl stop web.service
```
