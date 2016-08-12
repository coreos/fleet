## Unit states

fleet uses a _declarative model_ to evaluate unit state. This means that operations to change the state of units (e.g. `fleetctl` commands, or calls to the fleet API) change the desired state, rather than directly performing any state change. There are currently three cluster-level states for a unit:

- `inactive`: known by fleet, but not assigned to a machine
- `loaded`: assigned to a machine and loaded into systemd there, but not started
- `launched`: loaded into systemd, and fleet has called the equivalent of `systemctl start`

Units may only transition directly between these states. For example, for a unit to transition from `inactive` to `launched` it must first go state `loaded`.

The desired and last known states are exposed in the `DSTATE` and `STATE` columns of the output from `fleetctl list-unit-files`.

The `fleetctl` commands to act on units change the *desired state* of a unit. fleet itself is then responsible for performing the necessary state transitions to move a unit to the desired state. The following table explains the relationship between each `fleetctl` command and unit states.

| Command | Description | Desired State | Valid Previous States | Is an alias for |
|---------|-------------|--------------|-----|----|
| `fleetctl submit`  | Submits unit file into etcd registry | `inactive`  | `none` | |
| `fleetctl load`    | Submits and schedules unit file into machines' systemd but doesn't start it | `loaded` | `none` or `inactive` | `submit+load` |
| `fleetctl start`   | Submits, schedules and starts unit file| `launched`  | `none` or `inactive` or `loaded` | `submit+load+start` |
| `fleetctl stop`    | Stops scheduled unit file | `loaded`  | `launched` | |
| `fleetctl unload`  | Stops and unschedules unit file from machines' systemd | `inactive`| `launched` or `loaded` | `stop+unload` |
| `fleetctl destroy` | Stops, unschedules and removes unit file from etcd registry| `none` | `launched` or `loaded` or `inactive` | `stop+unload+destroy` |

`none` indicates that the unit has not yet been submitted to fleet at all (or it previously existed in fleet but was destroyed).

For example:
- if a unit is `inactive`, then `fleetctl start` will cause it to be `loaded` and then `launched`
- if a unit is `loaded`, then `fleetctl destroy` will cause it to be `inactive` and then `none`
- if a unit is `inactive`, then `fleetctl stop` is an invalid action

## systemd states

The other state associated with units in fleet is their systemd unit state. This will only exist for units which are assigned to a machine and known by systemd on that machine; i.e., they are in state `loaded` or `launched`.

The systemd state is composed of three subcomponents, exposed in `fleetctl list-units`. fleet retrieves this state directly from systemd and performs no manipulation before presenting it to the user; they correspond exactly to the respective output columns from the `systemctl list-units` command.

- `LOAD` (reflects whether the unit definition was properly loaded)
- `ACTIVE` (the high-level unit activation state, i.e. generalization of SUB)
- `SUB` (the low-level unit activation state, values depend on unit type)

By default, only the `ACTIVE` and `SUB` unit states are exposed by `fleetctl list-units`.
