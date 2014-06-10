# fleet API v1-rc.1

The primary goal of this document is to describe a new API for [fleet][fleet-gh]. This is a work-in-progress.

[fleet-gh]: https://github.com/coreos/fleet

## TODO

- reverse pagination
- name-matching filters for job/machine collections?
- request IDs
- flesh out MACHINE.networks
- should POST /commands be a transaction?
- signed unit files

## Media-Types

All entities used in communication with the v1 fleet API are represented by a unique media-type built on this template:

```
application/vnd.coreos.fleet.<ENTITY>+<FORMAT>
```

For example, the following media-type represents a json-formatted entity of type "units-v7":

```
application/vnd.coreos.fleet.units-v7+json
```

As the v1 API develops, new media-types will be introduced. Support for older media-types will be maintained as long as possible.

### Version

The version field in a media-type is comprised of a single major version numbers. A change in a version is intended to signify a new representation without any forwards- or backwards-compatibility guarantees.

### Format

The only two supported formats are `json` and `text`. All resources are not required to provide both formats. Other formats may be supported in the future.

## Version Negotiation

### Accept Headers

All responses must provide an entity of a `Content-Type` that fulfills the expectations set in the `Accept` header of the request. If no `Accept` header is provided in a request, the server may decide 

### Content-Type Headers

If a request is made or a response returned with a non-empty body, an appropriate  Content-Type header must be provided.

### URL Slug

An endpoint implementing the API described by this document is not required to have "v1" in its name, but it is encouraged. For example, "example.com:8080/fleet/v1/", "v1.fleet.example.com:8080/" and "example.com/" are all valid endpoints.

## Capability Discovery

As the v1 API develops over time, it is important that client can identify what capabilities are supported by a specific v1 API endpoint. This ability is built in to the v1 specification.

The /discover resource provides the ability for a client to programmatically discover what an endpoint with which it is communicating provides. Making an HTTP GET request against this resource will result in a document describing two things: what resources are available and what media types those resources support. For example:

```
GET /discover HTTP/1.1
Accept: application/vnd.coreos.fleet.discover-v1+json


HTTP/1.1 200 OK
Content-Length: N
Content-Type: application/vnd.coreos.fleet.discover-v1+json

{
	"units": {
		"link": "/units",
		"media-types": [
			"application/vnd.coreos.fleet.units-v1+json",
			"application/vnd.coreos.fleet.unit-v1+json",
			"application/vnd.coreos.fleet.unit-file-v1+text"
		]
	},
	"machines": {
		"link": "/machines",
		"media-types": [
			"application/vnd.coreos.fleet.machines-v1+json"
		]
	},
	"commands": {
		"link": "/commands",
		"media-types": [
			"application/vnd.coreos.fleet.commands-v1+json"
		]
	},
	"events": {
		"link": "/events",
		"media-types": [
			"application/vnd.coreos.fleet.events-v1+json"
		]
	},
	"history": {
		"link": "/history",
		"media-types": [
			"application/vnd.coreos.fleet.events-v1+json"
		]
	},
}
```

## Pagination

All collection resources use the same mechanism for paginating through entities.
A client may provide several query parameters to interact with this mechanism:

- **token**: An opaque string that represents a view of the collection at a given time. Using the same token will guarantee a consistent view of the collection across several paginated requests.
- **marker**: A string identifying the last-seen entity in a collection. All entities returned in a page must appear in the collection after the identified entity.
- **size**: An integer representing the maximum number of entities that should be returned in a single page.

Any paginated response to an HTTP GET will provide a set of URLs that should be used to navigate through the collection.

- **first**: Link to the first page of entities. This link will always be provided.
- **next**: Link to the next page of entities. This link will not be returned with the last page of entities.

If a "next" link is provided to the user, it is safe to assume that pagination should continue. If a "next" link is absent, the client should stop pagination.

All paginated responses must return a `Content-Location` header that provides a permanent link to the current page of entities.

### Example

The following demonstrates a series of paginated requests and responses against a fictional resource:

```
GET /cats HTTP/1.1
Accept: application/vnd.cats+json


HTTP/1.1 200 OK
Content-Location: /cats?token=14488&size=2
Content-Length: N
Content-Type: application/vnd.cats+json

{"cats": [{"id": "baxter"}, {"id": "charlie"}], "first": "/cats?token=14488&size=2", "next": "/cats?token=14488&size=2&marker=charlie"}


GET /cats?token=14488&size=2&marker=charlie HTTP/1.1
Accept: application/vnd.cats+json


HTTP/1.1 200 OK
Content-Location: /cats?token=14488&size=2&marker=charlie
Content-Length: N
Content-Type: application/vnd.cats+json

{"cats": [{"id": "michael"}, {"id": "prescott"}], "first": "/cats?token=14488&size=2", "next": "/cats?token=14488&size=1&marker=prescott"}


GET /cats?token=14488&size=2&marker=prescott HTTP/1.1
Accept: application/vnd.cats+json


HTTP/1.1 200 OK
Content-Location: /cats?token=14488&size=2&marker=prescott
Content-Length: N
Content-Type: application/vnd.cats+json

{"cats": [{"id":"russell"}], "first": "/cats?token=14488&size=2", "next": "/cats?token=14488&size=1&marker=russell"}


GET /cats?token=14488&size=2&marker=russell HTTP/1.1
Accept: application/vnd.cats+json


HTTP/1.1 200 OK
Content-Location: /cats?token=14488&size=2&marker=russell
Content-Length: N
Content-Type: application/vnd.cats+json

{"cats": [{"id":"timothy"}], "first": "/cats?token=14488&size=2"}
```

## Timestamps

All timestamp fields comply with IS0 8601.

## Entities

### Unit

- **name** (string): unique identifier of entity
- **hash** (string): SHA1 hash of corresponding unit-file
- **state** (string): fleet's state of the unit (inactive, loaded, or launched)
- **loadState** (string): systemd's LOAD state of the unit
- **activeState** (string): systemd's ACTIVE state of the unit
- **subState** (string): systemd's SUB state of the unit
- **machineID** (string): identifier of the machine to which this unit is scheduled

#### media-types
- application/vnd.coreos.fleet.unit-v1+json

### Unit-File

A unit-file entity is a systemd unit file with additional options defined by the fleet API.

#### media-types

- application/vnd.coreos.fleet.unit-file-v1+text

### Machine

- **id** (string): unique identifier of entitiy
- **metadata** (object): dictionary of key-value data published by the machine
- **network** (list): list of dictionaries representing the machine's configured networks

### Commands

Command entities implement a set of shared fields, but are also free to define custom fields. The shared fields are:

- **type** (string): classification of event entity

#### LoadUnit

Request that the unit be loaded on a machine in the cluster.

- **unitName** (string): unique identifier of unit

#### StartUnit

Request that the unit be started on its machine.
If this unit is not already loaded, this command will fail.

- **unitName** (string): unique identifier of unit

#### StopUnit

Request that the unit be started on its machine.
If this unit is not already loaded, this command will fail.

- **unitName** (string): unique identifier of unit

#### UnloadUnit

Request that the unit be unscheduled from its current machine.
If this unit is not already loaded, this command will fail.

- **unitName** (string): unique identifier of unit


### Events

Event entities implement a set of shared fields, but are also free to define custom fields. The shared fields are:

- **type** (string): classification of event entity

#### UnitCreated

A new unit was created.

- **unitName** (string): unique identifier of unit that was created

#### MachineLost

A machine left the cluster unexpectedly.

- **machineID** (string): unique identifier of machine that was lost


## Operations

### List Units

```
GET /units HTTP/1.1
Accept: application/vnd.coreos.fleet.units-v1+json


HTTP 1.1 200 OK
Content-Length: N
Content-Type: application/vnd.fleet.units-v1+json

{"units"": [UNIT, ... ], "first": URL, "next": URL}
```

##### Filters

- machineID
- unitState (fleet's internal state)
- subState (systemd state)
- activeState (systemd state)
- subState (systemd state)


### Create Unit

```
PUT /units/foo.service HTTP/1.1
Content-Type: application/vnd.coreos.fleet.unit-file-v1+text

[Service]
ExecStart=/usr/bin/sleep 1d


HTTP 1.1 201 Created
ETag: XXX
```

The ETag header in a successful response is a hash of the unit file.

Units are not mutable. In the event that a unit already exists, a `409 Conflict` will be returned.

### Destroy Units

```
DELETE /units/foo.service HTTP/1.1
If-Match: XXX

204 No Content
```

The If-Match header can be provided to ensure the entity being deleted has not changed since it was created.

### Manipulate Existing Units

Interaction with units in the cluster happens through COMMAND entities.

```
POST /commands HTTP/1.1
Content-Type: application/vnd.coreos.fleet.commands-v1+json

{"commands"": [COMMAND, ... ], "first": URL, "next": URL}
```

### List Machines

```
GET /machines HTTP/1.1


```

### Subscribe to Events

The "events" resource allows a client to subscribe to a filtered stream of event entities. This is a long-polling HTTP request.

```
GET /events HTTP/1.1


```

The token used for paginating collection resources may also be used here to begin receiving events from an absolute point in time.

##### Filters

- unitName
- machineID
- index

### Explore History

The "history" resource allows a client to inspect the historical event stream using a series of filters. This is intended to help a client understand what happened and how a given unit reached its current state.

```
GET /history HTTP/1.1


```

The token used for paginating collection resources may also be used here to limit the history to a point in time.