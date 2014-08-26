# Unit Files

fleet will schedule any valid service, socket, path or timer systemd unit to a machine in the cluster, taking into account a few special properties in the `[X-Fleet]` section. If you're new to using systemd unit files, check out the [Getting Started with systemd guide](https://coreos.com/docs/launching-containers/launching/getting-started-with-systemd).

## Unit Requirements

* Must be a supported unit type: `service`, `socket`, `device`, `mount`, `automount`, `timer`, `path`
* Each unit file must have a file extension corresponding to its respective unit type.

## fleet-specific Options

| Option Name | Description |
|---------------|-------------|
| `X-ConditionMachineID` | Require the unit be scheduled to the machine identified by the given string. |
| `X-ConditionMachineOf` | Limit eligible machines to the one that hosts a specific unit. |
| `X-ConditionMachineMetadata` | Limit eligible machines to those with this specific metadata. |
| `X-Conflicts` | Prevent a unit from being collocated with other units using glob-matching on the other unit names. |
| `Global` | Schedule this unit on all agents in the cluster. Should not be used with other options. | 

See [more information](https://github.com/coreos/fleet/blob/master/Documentation/scheduling.md) on these parameters and how they impact scheduling decisions.

Take the following as an example of how your `[X-Fleet]` section could be written:

```
[Unit]
Description=Some Monitoring Service

[Service]
ExecStart=/bin/monitorme

[X-Fleet]
X-ConditionMachineMetadata=location=chicago
X-Conflicts=monitor*
```

## systemd specifiers

When evaluating the `[X-Fleet]` section, fleet supports a subset of systemd's [specifiers][systemd specifiers] to perform variable substitution. The following specifiers are currently supported:

* `%n`
* `%N`
* `%p`
* `%i`

For the meaning of the specifiers, refer to the official [systemd documentation][systemd specifiers].

[systemd specifiers]: http://www.freedesktop.org/software/systemd/man/systemd.unit.html#Specifiers
