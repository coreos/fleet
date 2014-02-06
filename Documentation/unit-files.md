# Unit Files

fleet will schedule any valid service or socket systemd unit to a machine in the cluster, taking into account a few special properties in the `[X-Fleet]` section. If you're new to using systemd unit files, check out the [Getting Started with systemd guide](https://coreos.com/docs/launching-containers/launching/getting-started-with-systemd).

## Unit Requirements

* Only service and socket unit types are supported, and file names must have '.service' and '.socket' file extensions, respectively.
* Unit files must not have an [Install] section.

## fleet-specific Options

| Option Name | Description |
|---------------|-------------|
| `X-ConditionMachineBootID` | Require a job to be scheduled to a specific machine. The value of this option is the boot ID of a machine in the cluster. If no machine in the cluster has this boot ID, the job will not be scheduled. |
| `X-ConditionMachineOf` | Require a specific job be scheduled to a machine for it to be considered a candidate in scheduling. This allows a given job to 'follow' another around the system. |

Take the following as an example of how your `[X-Fleet]` section could be written:

```
[Unit]
Description=Some Monitoring Service

[Service]
ExecStart=/bin/monitorme

[X-Fleet]
X-ConditionMachineBootID=148a18ff-6e95-4cd8-92da-c9de9bb90d5a
```
