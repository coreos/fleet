# Scheduling Services

## Making Scheduling Decisions

The current method of making service placement decisions is incredibly simple. 
When a user requests a given service be started in the system, a JobOffer is created.
Agents react to this JobOffer by deciding if they are able to run the referenced Job, and if so, submitting a JobBid back to the Engine.
The Engine simply accepts the first bid that is submitted for a given offer and commits the schedule change.

**NOTE:** The current approach of accepting the first bid is only temporary - the Engine will make an effort to fairly schedule across the entire schedule in the near future.

Read more about [fleet's architecture and data model](architecture.md).

## User-Defined Requirements

##### Schedule unit to specific machine

The `X-ConditionMachineBootId` option of a unit file causes the system to schedule a unit to a machine with a boot ID matching the option's value.

The boot ID of each machine is currently published in the `MACHINE` column in the output of `fleetctl list-machines -l`.
One must use the entire boot ID when setting `X-ConditionMachineBootId` - the shortened ID returned by `fleetctl list-machines` without the `-l` flag is not acceptable.

It is important to note that a machine's boot ID is ephemeral and will change across reboots.

##### Schedule unit to machine with specific metadata

When calling `fleetctl start`, a user may provide a `--require` flag.
The value of this flag is a comma-delimited list of `<key>=<value>` items.

```
$ fleetctl start --require region=us-east-1,host-type=SSD
```

This requires an eligible machine to have at least the `region` and `host-type` keys set accordingly. A single key may also be defined multiple times:

```
$ fleetctl start --require region=us-east-1,region=us-east-2
```

This would allow a machine to match just one of the provided values to consider themselves capable of running a job.

A machine is not automatically configured with metadata.
A deployer may define machine metadata using the `metadata` [config option](configuration.md).

##### Schedule unit next to another unit

In order for a unit to be scheduled to the same machine as another unit, a unit file can define `X-ConditionMachineOf`.
The value of this option is the exact name of another unit in the system, which we'll call the target unit.

If the target unit is not found in the system, the follower unit will be considered unschedulable. 
Once the target unit is scheduled somewhere, the follower unit will be scheduled there as well.

Follower units will reschedule themselves around the cluster to ensure their `X-ConditionMachineOf` options are always fulfilled.

##### Schedule unit away from other unit(s)

The value of the `X-Conflicts` option is a [glob pattern](http://golang.org/pkg/path/#Match) defining which other units next to which a given unit must not be scheduled.

If a unit is scheduled to the system without an `X-Conflicts` option, other units' conflicts still take effect and prevent the new unit from being scheduled to machines where conflicts exist.