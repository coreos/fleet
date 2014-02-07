# Security

## Preview Release

The preview release of fleet doesn't currently perform any authentication or authorization for submitted units. This means that any client that can access your etcd cluster can possibly run arbitrary code on many of your machines very easily.

## Fast Follow Plans

Units submitted to fleet will be signed when submitted and verified on the machine before the unit is loaded and started.