# Security

## Preview Release

The preview release of fleet doesn't currently perform any authentication or authorization for submitted units. This means that any client that can access your etcd cluster can possibly run arbitrary code on many of your machines very easily.

You should avoid public access to etcd and instead run fleet [from your local laptop](using-the-client.md#get-up-and-running) with the `--tunnel` flag to run commands over an SSH tunnel. You can alias this flag for easier usage: `alias fleetctl=fleetctl --tunnel 10.10.10.10`.

## Fast Follow Plans

Units submitted to fleet will be signed when submitted and verified on the machine before the unit is loaded and started.