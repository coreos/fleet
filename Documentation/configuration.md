# Configuration

The fleet config file is formatted using [TOML](https://github.com/mojombo/toml/blob/master/versions/toml-v0.2.0.md). A [sample config][config] exists in the root of the fleet source code. The recognized options are documented below:

[config]: https://github.com/coreos/fleet/blob/master/fleet.conf.sample

#### verbosity

Increase the amount of log information. Acceptable values are 0, 1, and 2 - higher values are more verbose.

Default: 0

#### etcd_servers

Provide a custom set of etcd endpoints.

Default: ["http://127.0.0.1:4001"]

#### public_ip

IP address that should be published with the local Machine's state and any socket information.

Default: ""

#### metadata

Comma-delimited key/value pairs that are published with the local to the fleet registry. This data can be used directly by a client of fleet to make scheduling descisions. An example set of metadata could look like:  

	metadata="region=us-west,az=us-west-1"

Default: ""

#### agent_ttl

An Agent will be considered dead if it exceeds this amount of time to communicate with the Registry. The agent will attempt a heartbeat at half of this value.

Default: "30s"

## Development Settings

#### boot_id

Unique identifier of fleet instance.

Default: contents of file /proc/sys/kernel/random/boot_id

##### unit_prefix

Prefix to use when naming local systemd units. This prefix will never be exposed outside of this machine.

Default: ""
