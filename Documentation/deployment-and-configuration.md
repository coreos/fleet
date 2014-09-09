<i>Note: this document closely tracks the *master* branch of fleet. For information most accurate to the version of fleet you are running, please browse the documentation at a particular tagged release. Recent releases: [v0.6.2](https://github.com/coreos/fleet/tree/v0.6.2/Documentation) [v0.7.1](https://github.com/coreos/fleet/tree/v0.7.1/Documentation) [v0.8.0](https://github.com/coreos/fleet/tree/v0.8.0/Documentation)</i>

# Deploying fleet

Deploying `fleet` is as simple as dropping the `fleetd` binary on a machine with access to etcd and starting it.

Deploying `fleet` on CoreOS is even simpler: just run `systemctl start fleet`. The built-in configuration assumes each of your hosts is serving an etcd endpoint at the default location (http://127.0.0.1:4001). However, if your etcd cluster differs, you must make the corresponding configuration changes.

### etcd

Each `fleetd` daemon must be configured to talk to the same [etcd cluster][etcd]. By default, the `fleetd` daemon will connect to http://127.0.0.1:4001. Refer to the configuration documentation below for customization help.

`fleet` requires etcd be of version 0.3.0+.

[etcd]: https://coreos.com/docs/cluster-management/setup/getting-started-with-etcd

### systemd

The `fleetd` daemon communicates with systemd (v207+) running locally on a given machine. It requires D-Bus (v1.6.12+) to do this.

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

# Configuration

The `fleetd` daemon uses two sources for configuration parameters:

1. an INI-formatted config file ([sample][config])
2. environment variables

[config]: https://github.com/coreos/fleet/blob/master/fleet.conf.sample

fleet will look at `/etc/fleet/fleet.conf` for this config file by default. The `--config` flag may be passed to the `fleetd` binary to use a custom config file location. The options that may be set are defined below. Note that each of the options should be defined at the global level, outside of any INI sections.

Environment variables may also provide configuration options. Options provided in an environment variable will override the corresponding option provided in a config file. To use an environment variable, simply prefix the name of a given option with 'FLEET_', while uppercasing the rest of the name. For example, to set the `etcd_servers` option to 'http://192.0.2.12:4001' when running the fleetd binary:

```
$ FLEET_ETCD_SERVERS=http://192.0.2.12:4001 /usr/bin/fleetd
```

## General Options

#### verbosity

Increase the amount of log information. Acceptable values are 0, 1, and 2 - higher values are more verbose.

Default: 0

#### etcd_servers

Provide a custom set of etcd endpoints.

Default: ["http://127.0.0.1:4001"]

#### etcd_request_timeout

Amount of time in seconds to allow a single etcd request before considering it failed.

Default: 1.0

#### etcd_cafile, etcd_keyfile, etcd_certfile 

Provide TLS configuration when SSL certificate authentication is enabled in etcd endpoints

Default: ""

#### public_ip

IP address that should be published with the local Machine's state and any socket information.
If not set, fleetd will attempt to detect the IP it should publish based on the machine's IP routing information.

Default: ""

#### metadata

Comma-delimited key/value pairs that are published with the local to the fleet registry. This data can be used directly by a client of fleet to make scheduling descisions. An example set of metadata could look like:  

	metadata="region=us-west,az=us-west-1"

Default: ""

#### agent_ttl

An Agent will be considered dead if it exceeds this amount of time to communicate with the Registry. The agent will attempt a heartbeat at half of this value.

Default: "30s"

#### engine_reconcile_interval

Interval at which the engine should reconcile the cluster schedule in etcd.

Default: 2
