## coreinit - a distributed init system.

coreinit ties together systemd and etcd into a distributed init system.

### Assumptions

Machines have truly unique UUIDs and their metadata is perfectly cacheable.
If a machine changes IP addresses, etc it *must* have a new UUID.
