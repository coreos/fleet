# Configuration

The `fleet` daemon uses two sources for configuration parameters:

1. an INI-formatted config file ([sample][config])
2. environment variables

[config]: https://github.com/coreos/fleet/blob/master/fleet.conf.sample

fleet will look at `/etc/fleet/fleet.conf` for this config file by default. The `--config` flag may be passed to the fleet binary to use a custom config file location. The options that may be set are defined below. Note that each of the options should be defined at the global level, outside of any INI sections.

Environment variables may also provide configuration options. Options provided in an environment variable will override the corresponding option provided in a config file. To use an environment variable, simply prefix the name of a given option with 'FLEET_', while uppercasing the rest of the name. For example, to set the `etcd_servers` option to 'http://192.0.2.12:4001' when running the fleet binary:

```
$ FLEET_ETCD_SERVERS=http://192.0.2.12:4001 /usr/bin/fleet
```

## General Options

#### verbosity

Increase the amount of log information. Acceptable values are 0, 1, and 2 - higher values are more verbose.

Default: 0

#### etcd_servers

Provide a custom set of etcd endpoints.

Default: ["http://127.0.0.1:4001"]

#### public_ip

IP address that should be published with the local Machine's state and any socket information.
If not set, fleet will attempt to detect the IP it should publish based on the machine's IP routing information.

Default: ""

#### metadata

Comma-delimited key/value pairs that are published with the local to the fleet registry. This data can be used directly by a client of fleet to make scheduling descisions. An example set of metadata could look like:  

	metadata="region=us-west,az=us-west-1"

Default: ""

#### agent_ttl

An Agent will be considered dead if it exceeds this amount of time to communicate with the Registry. The agent will attempt a heartbeat at half of this value.

Default: "30s"

#### verify_units

Enable payload signature verification. Payloads without verifiable signatures will not be eligible to run on the local fleet server.

Default: false

#### authorized_keys_file

File containing public SSH keys that should be used to verify payload signatures.

Default: "~/.ssh/authorized_keys""

## Development Options

#### boot_id

Unique identifier of fleet instance.

Default: contents of file /proc/sys/kernel/random/boot_id

##### unit_prefix

Prefix to use when naming local systemd units. This prefix will never be exposed outside of this machine.

Default: ""
