<i>Note: this document closely tracks the *master* branch of fleet. For information most accurate to the version of fleet you are running, please browse the documentation at a particular tagged release. Recent releases: [v0.6.2](https://github.com/coreos/fleet/tree/v0.6.2/Documentation) [v0.7.1](https://github.com/coreos/fleet/tree/v0.7.1/Documentation) [v0.8.0](https://github.com/coreos/fleet/tree/v0.8.0/Documentation)</i>

## Unit states

There are currently three cluster-level states for a unit:

- `inactive`: known by fleet, but not assigned to a machine
- `loaded`: assigned to a machine and loaded into systemd there, but not started
- `launched`: loaded into systemd, and fleet has called the equivalent of `systemctl start`

Units may only transition directly between these states. For example, for a unit to transition from `inactive` to `launched` it must first go state `loaded`.

The desired and last known states are exposed in the `DSTATE` and `STATE` columns of the output from `fleetctl show-schedule`.

The `fleetctl` commands to act on units change the *desired state* of a unit. fleet itself is then responsible for performing the necessary state transitions to move a unit to the desired state. The following table explains the relationship between each `fleetctl` command and unit states.

| Command | Desired State | Valid Previous States |
|---------|--------------|-----|
| `fleetctl submit`  | `inactive`  | `(unknown)`
| `fleetctl load`    | `loaded` | `(unknown)` or `inactive` |
| `fleetctl start`   | `launched`  | `(unknown)` or `inactive` or `loaded` |
| `fleetctl stop`    | `loaded`  | `launched`
| `fleetctl unload`  | `inactive`| `launched` or `loaded` |
| `fleetctl destroy` | `(unknown)` | `launched` or `loaded` or `inactive` |


`(unknown)` indicates that the unit has not yet been submitted to fleet at all (or it previously existed in fleet but was destroyed).

For example:
- if a unit is `inactive`, then `fleetctl start` will cause it to be `loaded` and then `launched`
- if a unit is `loaded`, then `fleetctl destroy` will cause it to be `inactive` and then `(unknown)`
- if a unit is `inactive`, then `fleetctl stop` is an invalid action


## systemd states

The other state associated with units in fleet is their systemd unit state. This will only exist for units which are assigned to a machine and known by systemd on that machine; i.e., they are in state `loaded` or `launched`. 

The systemd state is composed of three subcomponents, exposed in `fleetctl list-units`. fleet retrieves this state directly from systemd and performs no manipulation before presenting it to the user; they correspond exactly to the respective output columns from the `systemctl list-units` command.

- `LOAD` (reflects whether the unit definition was properly loaded)
- `ACTIVE` (the high-level unit activation state, i.e. generalization of SUB)
- `SUB` (the low-level unit activation state, values depend on unit type)

By default, only the `ACTIVE` and `SUB` unit states are exposed by `fleetctl list-units`.
