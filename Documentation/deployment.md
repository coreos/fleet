# Deploying fleet

Getting fleet up and running simply requires you to start the fleet binary on a machine with access to etcd.

Deploying fleet on CoreOS is even simpler: just run `systemctl start etcd fleet`. The default configuration assumes each of your hosts is serving an etcd endpoint at the default location (http://127.0.0.1:4001). If your etcd cluster differs, you must make the corresponding [configuration changes](config).

For those running CoreOS on EC2, things are even easier! Use of the bootstrapping features built-in to CoreOS and etcd will result in fleet being started automatically.

### etcd

Each fleet daemon must be configured to talk to the same [etcd cluster][etcd]. By default, the fleet daemon will connect to 'http://127.0.0.1:4001'. Refer to the [configuration documentation][config] for customization help.

fleet requires etcd be of version 0.3.0+.

[etcd]: https://coreos.com/docs/cluster-management/setup/getting-started-with-etcd
[config]: configuration.md

### systemd

The fleet daemon communicates with systemd (v207+) running locally on a given machine. It requires D-Bus (v1.6.12+) to do this.

### SSH Keys

The `fleetctl` client tool uses SSH to interact with a fleet cluster. This means each client's public SSH key must be authorized to access each fleet machine.

Included in the [fleet source](../contrib/fleetctl-inject-ssh.sh) is a script that will help distribute SSH keys across a fleet cluster running on CoreOS. Using it is as simple as piping the contents of a public SSH key into the script:

```
cat ~/.ssh/id_rsa.pub | ./fleetctl-inject-ssh.sh simon
```

Any arguments past the first to `fleetctl-inject-ssh.sh` are passed directly to `fleetctl`.

```
cat ~/.ssh/id_rsa.pub | ./fleetctl-inject-ssh.sh simon --tunnel 19.12.0.33
```
