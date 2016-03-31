# Unit Files

Unit files are the primary means of interacting with fleet. They define what you want to do, and how fleet should do it.

fleet will schedule any valid service, socket, path or timer systemd unit to a machine in the cluster, taking into account a few special properties in the `[X-Fleet]` section. If you're new to using systemd unit files, check out the [Getting Started with systemd guide][systemd-guide].

## Unit Requirements

The unit name must be of the form `string.suffix` or `string@instance.suffix`, where:

* `string` must not be an empty string and can only contain alphanumeric characters and any of `:_.@-`. Formally, it must match the regular expression `[a-zA-Z0-9:_.@-]+`
* `instance` can be empty, and can only contain the same characters as are valid for `string`. Formally, it must match the regular expression `[a-zA-Z0-9:_.@-]*`
* `suffix` must be one of the following unit types: `service`, `socket`, `device`, `mount`, `automount`, `timer`, `path`

Note that these requirements are derived directly from systemd, with the only exception that the unit types are a subset of those supported by systemd.

## fleet-specific Options

| Option Name | Description |
|-------------|-------------|
| `MachineID` | Require the unit be scheduled to the machine identified by the given string. |
| `MachineOf` | Limit eligible machines to the one that hosts a specific unit. |
| `MachineMetadata` | Limit eligible machines to those with this specific metadata. |
| `Conflicts` | Prevent a unit from being collocated with other units using glob-matching on the other unit names. |
| `Global` | Schedule this unit on all agents in the cluster. A unit is considered invalid if options other than `MachineMetadata` are provided alongside `Global=true`. |

See [more information][unit-scheduling] on these parameters and how they impact scheduling decisions.

In versions of fleet <= 0.8.0, the following options are available. They are deprecated and should be migrated to the new options as soon as possible.

| Option Name | Description |
|-------------|-------------|
| `X-ConditionMachineID` | _Deprecated in v0.8.0 in favor of `MachineID`_ |
| `X-ConditionMachineOf` | _Deprecated in v0.8.0 in favor of `MachineOf`_ |
| `X-ConditionMachineMetadata` | _Deprecated in v0.8.0 in favor of `MachineMetadata`_ |
| `X-Conflicts` | _Deprecated in v0.8.0 in favor of `Conflicts`_ |

Take the following as an example of how your `[X-Fleet]` section could be written:

```
[Unit]
Description=Some Monitoring Service

[Service]
ExecStart=/bin/monitorme

[X-Fleet]
MachineMetadata=location=chicago
Conflicts=monitor*
```

## Template unit files

fleet provides support for using systemd's [instances][systemd instances] feature to dynamically create _instance_ units from a common _template_ unit file. This allows you to have a single unit configuration and easily and dynamically create new instances of the unit as necessary.

To use instance units, simply create a unit file whose name matches the `<name>@.<suffix>` format - for example, `hello@.service` - and submit it to fleet. You can then instantiate units by creating new units that match the instance pattern `<name>@<instance>.<suffix>` - in this case, for example, `hello@world.service` or `hello@1.service` - and fleet will automatically utilize the relevant template unit. For a detailed example, see the [example deployment][example-deployment].

When working with instance units, it is strongly recommended that all units be _entirely homogenous_. This means that any unit created as, say, `foo@1.service`, should be created only from the unit named `foo@.service`. This homogeneity will be enforced by the fleet API in future.

## systemd specifiers

When evaluating the `[X-Fleet]` section, fleet supports a subset of systemd's [specifiers][systemd specifiers] to perform variable substitution. The following specifiers are currently supported:


| Specifier   | Description              |
|-------------|--------------------------|
|    `%n`     | Full unit name           |
|    `%N`     | Unescaped full unit name |
|    `%p`     | Unescaped prefix name    |
|    `%i`     | Instance name            |


For more information, refer to the official [systemd documentation][systemd specifiers].

# Unit Scheduling

When working with units, fleet distinguishes between two types of units: _non-global_ (the default) and _global_. (A global unit is one with `Global=true` in its `X-Fleet` section, as mentioned above).

Non-global units are scheduled by the fleet engine - the engine is responsible for deciding where they should be placed in the cluster. 

Global units can run on every possible machine in the fleet cluster.
While global units are not scheduled through the engine, fleet agents still check the `MachineMetadata` option before starting them.
Other options are ignored.

For more details on the specific behavior of the engine, read more about [fleet's architecture and data model][fleet-architecture].

## User-Defined Requirements

For non-global units, several different directives are available to control the engine's scheduling decision.

##### Schedule unit to specific machine

The `MachineID` option of a unit file causes the system to schedule a unit to a machine identified by the option's value.

The ID of each machine is currently published in the `MACHINE` column in the output of `fleetctl list-machines -l`.
One must use the entire ID when setting `MachineID` - the shortened ID returned by `fleetctl list-machines` without the `-l` flag is not acceptable.

fleet depends on its host to generate an identifier at `/etc/machine-id`, which is handled today by systemd.
Read more about machine IDs in the [official systemd documentation][machine-id].

##### Schedule unit to machine with specific metadata

The `MachineMetadata` option of a unit file allows you to set conditional metadata required for a machine to be elegible.

```ini
[X-Fleet]
MachineMetadata="region=us-east-1" "diskType=SSD"
```

This requires an eligible machine to have at least the `region` and `diskType` keys set accordingly. This logic could be represented as follows:

```sql
region=us-east-1 AND diskType=SSD
```

A single key may also be defined multiple times, in which case only one of the conditions needs to be met:

```ini
[X-Fleet]
MachineMetadata=region=us-east-1
MachineMetadata=region=us-west-1
```

This would allow a machine to match just one of the provided values to be considered eligible to run. This logic could be represented as follows:

```sql
region=us-east-1 OR region=us-west-1
```

If we combine two previous examples in one:

```ini
[X-Fleet]
MachineMetadata="region=us-east-1" "diskType=SSD"
MachineMetadata=region=us-west-1
```

the logic would be as follows:

```sql
diskType=SSD AND (region=us-east-1 OR region=us-west-1)
```

Grouping metadata pairs onto separate lines has no affect on the matching logic:

```ini
[X-Fleet]
MachineMetadata="region=us-east-1" "job=foo"
MachineMetadata="region=us-west-1" "job=bar"
```

will be interpreted as:

```sql
(job=foo OR job=bar) AND (region=us-east-1 OR region=us-west-1)
```

The previous example schedules at most one unit across your cluster, depending on the first satisfied requirement. If you add `Global=true`:

```ini
[X-Fleet]
Global=true
MachineMetadata="region=us-east-1" "diskType=SSD"
MachineMetadata=region=us-west-1
```

then fleet will schedule this unit on all machines which meet these requirements:

```sh
$ fleetctl list-machines
MACHINE         IP         METADATA
282f949f...     10.10.20.1 diskType=SSD,region=us-east-1
f139c5a6...     10.10.20.2 region=us-east-1
fd1d3e94...     10.0.0.1   diskType=SSD,region=us-west-1
$ fleetctl list-units
UNIT            MACHINE                 ACTIVE  SUB
app.service     282f949f.../10.10.20.1  active  running
app.service     fd1d3e94.../10.0.0.1    active  running
```

A machine is not automatically configured with metadata.
A deployer may define machine metadata using the `metadata` [config option][config-option].

##### Schedule unit next to another unit

In order for a unit to be scheduled to the same machine as another unit, a unit file can define `MachineOf`.
The value of this option is the exact name of another unit in the system, which we'll call the target unit.

If the target unit is not found in the system, the follower unit will be considered unschedulable. 
Once the target unit is scheduled somewhere, the follower unit will be scheduled there as well.

Follower units will reschedule themselves around the cluster to ensure their `MachineOf` options are always fulfilled.

Note that currently `MachineOf` _cannot_ be a bidirectional dependency: i.e., if unit `foo.service` has `MachineOf=bar.service`, then `bar.service` must not have a `MachineOf=foo.service`, or fleet will be unable to schedule the units.

##### Schedule unit away from other unit(s)

The value of the `Conflicts` option is a [glob pattern][glob-pattern] defining which other units next to which a given unit must not be scheduled. A unit may have multiple `Conflicts` options.

If a unit is scheduled to the system without an `Conflicts` option, other units' conflicts still take effect and prevent the new unit from being scheduled to machines where conflicts exist.

##### Dynamic requirements

fleet supports several [systemd specifiers][systemd-specifiers] to allow requirements to be dynamically determined based on a Unit's name. This means that the same unit can be used for multiple Units and the requirements are dynamically substituted when the Unit is scheduled.

For example, a Unit by the name `foo.service`, whose unit contains the following snippet:

```ini
[X-Fleet]
MachineOf=%p.socket
```

would result in an effective `MachineOf` of `foo.socket`. Using the same unit snippet with a Unit called `bar.service`, on the other hand, would result in an effective `MachineOf` of `bar.socket`.

[config-option]: deployment-and-configuration.md#metadata
[systemd-guide]: https://github.com/coreos/docs/blob/master/os/getting-started-with-systemd.md
[systemd instances]: http://0pointer.de/blog/projects/instances.html
[systemd specifiers]: http://www.freedesktop.org/software/systemd/man/systemd.unit.html#Specifiers
[fleet-architecture]: architecture.md
[machine-id]: http://www.freedesktop.org/software/systemd/man/machine-id.html
[glob-pattern]: http://golang.org/pkg/path/#Match
[unit-scheduling]: #unit-scheduling
[example-deployment]: examples/example-deployment.md#service-files
[systemd-specifiers]: #systemd-specifiers
