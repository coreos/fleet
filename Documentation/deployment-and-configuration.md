# Deploying fleet

Deploying `fleet` is as simple as dropping the `fleetd` binary on a machine with access to etcd and starting it.

Deploying `fleet` on CoreOS is even simpler: just run `systemctl start fleet`. The built-in configuration assumes each of your hosts is serving an etcd endpoint at one of the default locations (http://127.0.0.1:4001 or http://127.0.0.1:2379). However, if your etcd cluster differs, you must make the corresponding configuration changes.

## etcd

Each `fleetd` daemon must be configured to talk to the same [etcd cluster][etcd]. By default, the `fleetd` daemon will connect to either http://127.0.0.1:4001 or http://127.0.0.1:2379, depending on which endpoint responds. Refer to the configuration documentation below for customization help.

`fleet` requires etcd be of version 0.3.0+.

[etcd]: https://coreos.com/docs/cluster-management/setup/getting-started-with-etcd

## systemd

The `fleetd` daemon communicates with systemd (v207+) running locally on a given machine. It requires D-Bus (v1.6.12+) to do this.

## SSH Keys

The `fleetctl` client tool uses SSH to interact with a fleet cluster. This means each client's public SSH key must be authorized to access each `fleet` machine.

Authorizing a public SSH key is typically as easy as appending it to the user's `~/.ssh/authorized_keys` file. This may not be true on your systemd, though. If running CoreOS, use the built-in `update-ssh-keys` utility - it helps manage multiple authorized keys.

To make things incredibly easy, included in the [fleet source](../scripts/fleetctl-inject-ssh.sh) is a script that will distribute SSH keys across a fleet cluster running on CoreOS. Simply pipe the contents of a public SSH key into the script:

```
cat ~/.ssh/id_rsa.pub | ./fleetctl-inject-ssh.sh simon
```

All but the first argument to `fleetctl-inject-ssh.sh` are passed directly to `fleetctl`.

```
cat ~/.ssh/id_rsa.pub | ./fleetctl-inject-ssh.sh simon --tunnel 19.12.0.33
```

## API

fleet's API is served using systemd socket activation.
At service startup, systemd passes fleet a set of file descriptors, preventing fleet from having to care on which interfaces it's serving the API.
The configuration of these interfaces is managed through a [systemd socket unit][socket-unit].

[socket-unit]: http://www.freedesktop.org/software/systemd/man/systemd.socket.html

CoreOS ships a socket unit for fleet (`fleet.socket`) which binds to a Unix domain socket, `/var/run/fleet.sock`.
To serve the fleet API over a network address, simply extend or replace this socket unit.
For example, writing the following drop-in to `/etc/systemd/system/fleet.socket.d/30-ListenStream.conf` would enable fleet to be reached over the local port `49153` in addition to `/var/run/fleet.sock`:

```
[Socket]
ListenStream=127.0.0.1:49153
```

After you've written the file, call `systemctl daemon-reload` to load the new drop-in, followed by `systemctl stop fleet.service; systemctl restart fleet.socket; systemctl start fleet.service`.

Once the socket is running, the fleet API will be available at `http://${ListenStream}/fleet/v1`, where `${ListenStream}` is the value of the `ListenStream` option used in your socket file.
This endpoint is accessible directly using tools such as curl and wget, or you can use fleetctl like so: `fleetctl --endpoint http://${ListenStream} <command>`.
For more information, see the [official API documentation][api-doc].

[api-doc]: https://github.com/coreos/fleet/blob/master/Documentation/api-v1.md

# Configuration

The `fleetd` daemon uses two sources for configuration parameters:

1. an INI-formatted config file ([sample][config])
2. environment variables

[config]: https://github.com/coreos/fleet/blob/master/fleet.conf.sample

fleet will look at `/etc/fleet/fleet.conf` for this config file by default. The `--config` flag may be passed to the `fleetd` binary to use a custom config file location. The options that may be set are defined below. Note that each of the options should be defined at the global level, outside of any INI sections.

Environment variables may also provide configuration options. Options provided in an environment variable will override the corresponding option provided in a config file. To use an environment variable, simply prefix the name of a given option with 'FLEET_', while uppercasing the rest of the name. For example, to set the `etcd_servers` option to 'http://192.0.2.12:2379' when running the fleetd binary:

```
$ FLEET_ETCD_SERVERS=http://192.0.2.12:2379 /usr/bin/fleetd
```

## General Options

#### verbosity

Enable debug logging by setting this to an integer value greater than zero.
Only a single debug level exists, so all values greater than zero are considered equivalent.

Default: 0

#### etcd_servers

Provide a custom set of etcd endpoints.

Default: "http://127.0.0.1:4001,http://127.0.0.1:2379"

#### etcd_request_timeout

Amount of time in seconds to allow a single etcd request before considering it failed.

Default: 1.0

#### etcd_cafile, etcd_keyfile, etcd_certfile 

Provide TLS configuration when SSL certificate authentication is enabled in etcd endpoints

Default: ""

#### etcd_username, etcd_password

Provide credentials when [authentication](https://github.com/coreos/etcd/blob/master/Documentation/authentication.md) is enabled in etcd endpoints.

Default: ""

#### etcd_key_prefix

Keyspace path for fleet data in etcd.

Default: "/_coreos.com/fleet/"

#### public_ip

IP address that should be published with the local Machine's state and any socket information.
If not set, fleetd will attempt to detect the IP it should publish based on the machine's IP routing information.

Default: ""

#### metadata

Comma-delimited key/value pairs that are published with the local to the fleet registry. This data can be used directly by a client of fleet to make scheduling decisions. An example set of metadata could look like:  

	metadata="region=us-west,az=us-west-1"

Default: ""

#### agent_ttl

An Agent will be considered dead if it exceeds this amount of time to communicate with the Registry. The agent will attempt a heartbeat at half of this value.

Default: "30s"

#### engine_reconcile_interval

Interval in seconds at which the engine should reconcile the cluster schedule in etcd.

Default: 2
