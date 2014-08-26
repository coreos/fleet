# Security

## Preview Release

Current releases of fleet don't currently perform any authentication or authorization for submitted units. This means that any client that can access your etcd cluster can potentially run arbitrary code on many of your machines very easily.

## Securing etcd

You should avoid public access to etcd and instead run fleet [from your local laptop](using-the-client.md#get-up-and-running) with the `--tunnel` flag to run commands over an SSH tunnel. You can alias this flag for easier usage: `alias fleetctl=fleetctl --tunnel 10.10.10.10` - or use the environment variable `FLEETCTL_TUNNEL`.

## Other Notes

Since it interacts directly with systemd over D-Bus, the fleetd daemon must be run with elevated privileges (i.e. as root) in order to perform operations like starting and stopping services. From the [systemd D-Bus documentation](http://www.freedesktop.org/wiki/Software/systemd/dbus/):

> In contrast to most of the other services of the systemd suite PID 1 does not use PolicyKit for controlling access to privileged operations, but relies exclusively on the low-level D-Bus policy language. (This is done in order to avoid a cyclic dependency between PolicyKit and systemd/PID 1.) This means that sensitive operations exposed by PID 1 on the bus are generally not available to unprivileged processes directly.
