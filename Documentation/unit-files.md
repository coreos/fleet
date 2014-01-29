# Unit Files

coreinit will schedule any valid service or socket systemd unit to a machine in the cluster, taking into account a few special properties in the `[X-Coreinit]` section. If you're new to using systemd unit files, check out the [Getting Started with systemd guide](https://coreos.com/docs/launching-containers/launching/getting-started-with-systemd).

## Unit Requirements

* Only service and socket unit types are supported, and file names must have '.service' and '.socket' file extensions, respectively.
* Unit files must not have an [Install] section.

## coreinit-specific Options

| Option Name | Description |
|---------------|-------------|
| `X-Coreinit-Provides` | List of services the unit provides or what roles it may fill. The format of the value is a comma-delimited list of strings. These values are not currently published anywhere. |
| `X-Coreinit-MachineSingleton` | Boolean controlling whether multiple copies of this service can run on a single machine. coreinit tests each individual token in `X-Coreinit-Provides` to decide whether a conflicting service is on a given machine. Currently, the string 'true' is the only value that qualifies as the boolean value true. |
| `X-Coreinit-Peers` | List of unit file names that must be scheduled to a given machine in order for this unit to be scheduled there. This allows a unit to 'follow' another around the system. The format of the value is a comma-delimited list of strings |

Take the following as an example of how your `[X-Coreinit]` section could be written:

```
[Unit]
Description=...

[Service]
ExecStart=...

[X-Coreinit]
X-Coreinit-Provides=webservice
X-Coreinit-MachineSingleton=true
```