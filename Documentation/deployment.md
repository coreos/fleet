# Deploying coreinit

Deploying `coreinit` is as simple as dropping a binary on a machine with access to etcd and starting it.

Deploying `coreinit` on CoreOS is even simpler: just run `systemctl start coreinit`. The built-in configuration assumes each of your hosts is serving an etcd endpoint at the default location (http://127.0.0.1:4001). However, if your etcd cluster differs, you must make the corresponding configuration changes.

### etcd

Each `coreinit` daemon must be configured to talk to the same [etcd cluster][etcd]. By default, the `coreinit` daemon will connect to 'http://127.0.0.1:4001. Refer to the [configuration documentation][config] for customization help.

[etcd]: https://coreos.com/docs/cluster-management/setup/getting-started-with-etcd
[config]: configuration.md

### systemd

The `coreinit` daemon communicates with systemd (v207+) running locally on a given machine. It requires D-Bus (v1.6.12+) to do this.

### SSH Keys

The `corectl` client tool uses SSH to interact with a coreinit cluster. This means each client's public SSH key must be authorized to access each `coreinit` machine.

Included in the [coreinit source](../contrib/corectl-inject-ssh.sh) is a script that will help distribute SSH keys across a coreinit cluster running on CoreOS. Using it is as simple as piping the contents of a public SSH key into the script:

```
cat ~/.ssh/id_rsa.pub | ./corectl-inject-ssh.sh simon
```

Any arguments past the first to `corectl-inject-ssh.sh` are passed directly to `corectl`.

```
cat ~/.ssh/id_rsa.pub | ./corectl-inject-ssh.sh simon --tunnel 19.12.0.33
```