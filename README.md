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
