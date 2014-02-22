# Unit Files

fleet will schedule any valid service or socket systemd unit to a machine in the cluster, taking into account a few special properties in the `[X-Fleet]` section. If you're new to using systemd unit files, check out the [Getting Started with systemd guide](https://coreos.com/docs/launching-containers/launching/getting-started-with-systemd).

## Unit Requirements

* Only service and socket unit types are supported, and file names must have '.service' and '.socket' file extensions, respectively.
* Unit files must not have an [Install] section.

## fleet-specific Options

| Option Name | Description |
|---------------|-------------|
| `X-ConditionMachineBootID` | Require the unit be scheduled to a specific machine defined by given boot ID. |
| `X-ConditionMachineOf` | Limit eligible machines to the one that hosts a specific unit. |
| `X-Conflicts` | Prevent a unit from being collocated with other units using glob-matching on the other unit names. |

See [more information](scheduling.md) on these parameters and how they impact scheduling decisions.

Take the following as an example of how your `[X-Fleet]` section could be written:

```
[Unit]
Description=Some Monitoring Service

[Service]
ExecStart=/bin/monitorme

[X-Fleet]
X-ConditionMachineBootID=148a18ff-6e95-4cd8-92da-c9de9bb90d5a
```
