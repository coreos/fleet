## Unit states

There are currently three cluster-level states for a unit:

- `inactive`: known by flt, but not assigned to a machine
- `loaded`: assigned to a machine and loaded into systemd there, but not started
- `launched`: loaded into systemd, and flt has called the equivalent of `systemctl start`

Units may only transition directly between these states. For example, for a unit to transition from `inactive` to `launched` it must first go state `loaded`.

The desired and last known states are exposed in the `DSTATE` and `STATE` columns of the output from `fltctl list-unit-files`.

The `fltctl` commands to act on units change the *desired state* of a unit. flt itself is then responsible for performing the necessary state transitions to move a unit to the desired state. The following table explains the relationship between each `fltctl` command and unit states.

| Command | Desired State | Valid Previous States |
|---------|--------------|-----|
| `fltctl submit`  | `inactive`  | `(unknown)`
| `fltctl load`    | `loaded` | `(unknown)` or `inactive` |
| `fltctl start`   | `launched`  | `(unknown)` or `inactive` or `loaded` |
| `fltctl stop`    | `loaded`  | `launched`
| `fltctl unload`  | `inactive`| `launched` or `loaded` |
| `fltctl destroy` | `(unknown)` | `launched` or `loaded` or `inactive` |


`(unknown)` indicates that the unit has not yet been submitted to flt at all (or it previously existed in flt but was destroyed).

For example:
- if a unit is `inactive`, then `fltctl start` will cause it to be `loaded` and then `launched`
- if a unit is `loaded`, then `fltctl destroy` will cause it to be `inactive` and then `(unknown)`
- if a unit is `inactive`, then `fltctl stop` is an invalid action


## systemd states

The other state associated with units in flt is their systemd unit state. This will only exist for units which are assigned to a machine and known by systemd on that machine; i.e., they are in state `loaded` or `launched`. 

The systemd state is composed of three subcomponents, exposed in `fltctl list-units`. flt retrieves this state directly from systemd and performs no manipulation before presenting it to the user; they correspond exactly to the respective output columns from the `systemctl list-units` command.

- `LOAD` (reflects whether the unit definition was properly loaded)
- `ACTIVE` (the high-level unit activation state, i.e. generalization of SUB)
- `SUB` (the low-level unit activation state, values depend on unit type)

By default, only the `ACTIVE` and `SUB` unit states are exposed by `fltctl list-units`.
