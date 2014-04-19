## fleet User API

This documented is intended to drive the design of fleet's user-facing API.
This is just a starting point, expect most of it to change.

## Spec

A user can interact with the API by directly calling commands, or by subscribing to events.

This spec will be published inside of the API itself using https://github.com/rwl/go-endpoints.

### Objects

The following objects will be used in many commands and events.
They are defined here for convenience.

#### Job

```
{
    "name" string,
    "state" string,
    "unit_hash" string,
    "unit_state" UnitState
}
```

#### Unit

```
{
	"file" string,
}
```

#### UnitState

```
{
	"load_state" string,
	"active_state" string,
	"sub_state" string,
}
```

### Commands

For commands that are indicated as non-blocking, the client is expected to subscribe to JobStateChanged events or poll GetJob to determine the outcome of their request.

#### CreateJob

Create a new Job object in fleet, similar to `fleetctl submit`.

* The job will be in the "inactive" state after creation.
* The name used to create this Job must be unique.

#### DestroyJob

Remove an existing Job from fleet, similar to `fleetctl destroy`.

#### ListJobs

Describe all known Job objects, similar to `fleetctl list-units`.

* Filters supported: state, glob-match name (i.e. foo.*.service)

#### GetJob

Describe the current state of a Job object, similar to `fleetctl status`.

#### LoadJob

Request that the system schedule an existing job to an Agent, similar to `fleetctl load`.

* The job will transition from "inactive" to "loaded" state.
* This is a non-blocking operation.

#### UnloadJob

Request that the system discard the scheduling of an existing job, similar to `fleetctl unload`.

* The job will transition from "loaded" to "inactive" state.
* This is a non-blocking operation.

#### StartJob

Request that the system schedule an existing job to an Agent, similar to `fleetctl start`.

* The job will transition from "loaded" to "launched" state.
* This is a non-blocking operation.

#### StopJob

Request that the system schedule an existing job to an Agent, similar to `fleetctl stop`.

* The job will transition from "launched" to "loaded" state.
* This is a non-blocking operation.

### Events

The following events are delivered to a client over a long-running HTTP stream.
A client must initialize their event stream by calling Subscribe,

#### Subscribe

Ask that the API stream all events matching a set of filters back to the client.

* Filters: glob-matching job name, event type

#### Unsubscribe

Inform the API that events should no longer be streamed over the existing connection.
Use this to re-establish a stream with different filters using the same connection.

#### JobStateChanged

The `state` of a given Job has changed from `prev_state` to `state`.

```
{
	"name" string,
	"prev_state" string,
	"state" string,
}
```

#### PayloadStateChanged

The `payload_state` of a given Job has changed from `prev_payload_state` to `payload_state`.

```
{
	"name" string,
	"prev_payload_state" PayloadState,
	"payload_state" PayloadState,
}
```

#### JobDestroyed

A Job has been removed from the system.

```
{
	"name" string,
}
```

#### MachineJoined

A Machine has joined the cluster.

```
{
	"boot_id" string,
}
```

#### MachineLeft

A Machine has left the cluster.

```
{
	"boot_id" string,
}
```
