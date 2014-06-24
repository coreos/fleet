# Deploying fleet

Deploying `fleet` is as simple as dropping a binary on a machine with access to etcd and starting it.

Deploying `fleet` on CoreOS is even simpler: just run `systemctl start fleet`. The built-in configuration assumes each of your hosts is serving an etcd endpoint at the default location (http://127.0.0.1:4001). However, if your etcd cluster differs, you must make the corresponding configuration changes.

### etcd

Each `fleet` daemon must be configured to talk to the same [etcd cluster][etcd]. By default, the `fleet` daemon will connect to http://127.0.0.1:4001. Refer to the [configuration documentation][config] for customization help.

`fleet` requires etcd be of version 0.3.0+.

[etcd]: https://coreos.com/docs/cluster-management/setup/getting-started-with-etcd
[config]: https://github.com/coreos/fleet/blob/master/Documentation/configuration.md

### systemd

The `fleet` daemon communicates with systemd (v207+) running locally on a given machine. It requires D-Bus (v1.6.12+) to do this.

### SSH Keys

The `fleetctl` client tool uses SSH to interact with a fleet cluster. This means each client's public SSH key must be authorized to access each `fleet` machine.

Authorizing a public SSH key is typically as easy as appending it to the user's `~/.ssh/authorized_keys` file. This may not be true on your systemd, though. If running CoreOS, use the built-in `update-ssh-keys` utility - it helps manage multiple authorized keys.

To make things incredibly easy, included in the [fleet source](../contrib/fleetctl-inject-ssh.sh) is a script that will distribute SSH keys across a fleet cluster running on CoreOS. Simply pipe the contents of a public SSH key into the script:

```
cat ~/.ssh/id_rsa.pub | ./fleetctl-inject-ssh.sh simon
```

All but the first argument to `fleetctl-inject-ssh.sh` are passed directly to `fleetctl`.

```
cat ~/.ssh/id_rsa.pub | ./fleetctl-inject-ssh.sh simon --tunnel 19.12.0.33
```
