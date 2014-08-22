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
