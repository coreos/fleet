# fleet API v1

The fleet API allows you to manage the state of the cluster using JSON over HTTP.

## Managing Units

Create and modify Unit entities to communicate to fleet the desired state of the cluster.
This simply declares what *should* be happening; the backend system still has to react to the changes in this desired state.
The actual state of the system is communicated with UnitState entities.

### Unit Entity

- **name**: (readonly) unique identifier of entity
- **options**: list of UnitOption entities
- **desiredState**: state the user wishes the Unit to be in ("inactive", "loaded", or "launched")
- **currentState**: (readonly) state the Unit is currently in (same possible values as desiredState)
- **machineID**: ID of machine to which the Unit is scheduled

A UnitOption represents a single option in a systemd unit file.

- **section**: name of section that contains the option (e.g. "Unit", "Service", "Socket")
- **name**: name of option (e.g. "BindsTo", "After", "ExecStart")
- **value**: value of option (e.g. "/usr/bin/docker run busybox /bin/sleep 1000")

### Create a Unit

#### Request

Create a Unit by passing a partial Unit entity to the /units resource.
The options and desiredState fields are required, and all other Unit fields will be ignored.

The base request looks like this:

```
PUT /fleet/v1/units/<name> HTTP/1.1

{"desiredState": <state>, "options": [<option>, ...]}
```

For example, creating and launching a new unit "foo.service" could be done like so:

```
PUT /fleet/v1/units/foo.service HTTP/1.1

{
  "desiredState": "launched",
  "options": [{"section": "Service", "name": "ExecStart", "value": "/usr/bin/sleep 3000"}]
}
```

**Note:** If the unit's name field is set in the request body, it must match the
name in the PUT /units/<name> request.

#### Response

A success is indicated by a `201 Created` status code, but no response body.

Attempting to create an entity without options will return a `409 Conflict` status code.

Attempting to create an invalid entity will result in a `400 Bad Request` response.

### Modify a Unit's desiredState

#### Request


Modify the desired state of an existing Unit by providing a partial entity.
The only required field is desiredState:

```
PUT /fleet/v1/units/<name> HTTP/1.1

{"desiredState": <state>}
```

For example, unloading an existing Unit called "bar.service" could look like this:

```
PUT /fleet/v1/units/bar.service HTTP/1.1

{
  "desiredState": "inactive"
}
```

If the Unit's name field is set in the request body, it must match the name in the URL.

#### Response

A success is indicated by a `204 No Content`.

Attempting to modify a Unit with an invalid entity will result in a `400 Bad Request` response.

### List Units

Explore a paginated collection of Unit entities.

#### Request

```
GET /fleet/v1/units HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will have a `200 OK` status code and body containing a single page of zero or more Unit entities.

### Get a Unit

View a particular Unit entity.

#### Request

```
GET /fleet/v1/units/<name> HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will have a `200 OK` status code and body containing a single Unit entity.

If the requested Unit does not exist, a `404 Not Found` will be returned.

### Destroy a Unit

Completely remove a Unit from fleet.

#### Request

```
DELETE /fleet/v1/units/<name> HTTP/1.1
```

#### Response

A successful response is indicated by a `204 No Content`.

If the indicated Unit does not exist, a `404 Not Found` will be returned.

## Current Unit State

Whereas Unit entities represent the desired state of units known by fleet, UnitStates represent the current states of units actually running in the cluster.
The information reported by UnitStates will not always align perfectly with the Units, as there is a delay between the declaration of desired state and the backend system making all of the necessary changes.

### UnitState Entity

- **name**: unique identifier of entity
- **hash**: SHA1 hash of underlying unit file
- **machineID**: ID of machine from which this state originated
- **systemdLoadState**: load state as reported by systemd
- **systemdActiveState**: active state as reported by systemd
- **systemdSubState**: sub state as reported by systemd

### List Unit State

Explore a paginated collection of UnitState entities.

#### Request

```
GET /fleet/v1/state HTTP/1.1
```

The request must not have a body.

The request may be filtered using two query parameters:
- **machineID**: filter all UnitState objects to those originating from a specific machine
- **unitName**: filter all UnitState objects to those related to a specific unit

#### Response

A successful response will contain a single page of zero or more UnitState entities.

## Machines

### Machine Entity

A Machine represents a host in the cluster.
It uses the host's [machine-id][systemd-machine-id] as a unique identifier.

- **id**: unique identifier of Machine entity
- **primaryIP**: IP address that should be used to communicate with this host
- **metadata**: dictionary of key-value data published by the machine

### List Machines

Explore a paginated collection of Machine entities.

#### Request

```
GET /fleet/v1/machines HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a page of zero or more Machine entities.

## Capability Discovery

The v1 fleet API is described by a [discovery document][disco]. Users should generate their client bindings from this document using the appropriate language generator.
This document is available in the [fleet source][schema] and served directly from the API itself, at the `/discovery` endpoint.
Note that this discovery document intentionally ships with an unusable `rootUrl`; clients *must* initialize this as appropriate.

An extremely simplified example client can be found [here][example].

## Media Types

All API requests and responses use the `application/json` media type.
New media types may be introduced in the future.

## Pagination

If a collection is large enough to warrant a paginated response, it will return a `nextPageToken` field in its response body.
To retrieve the next page of entities, a client must make a subsequent HTTP request with a single `nextPageToken` query parameter set to the value received in a response body.
If a paginated response does not contain a `nextPageToken` field, a client may safely assume no more entities are available.

### Pagination Example

The following series of HTTP request/response pairs demonstrates how pagination works against a fictional resource:

```
GET /fleet/v1/cats HTTP/1.1


HTTP/1.1 200 OK

{"cats": [{"id": "baxter"}, {"id": "charlie"}], "nextPageToken": "8fefec2c"}
```

```
GET /fleet/v1/cats?nextPageToken=8fefec2c HTTP/1.1


HTTP/1.1 200 OK

{"cats": [{"id": "michael"}, {"id": "prescott"}], "nextPageToken": "cbb06916"}
```

```
GET /fleet/v1/cats?nextPageToken=cbb06916 HTTP/1.1


HTTP/1.1 200 OK

{"cats": [{"id":"timothy"}]}
```

## Error Communication

400- and 500-level API responses may return JSON-encoded error entities.
The response will have an `application/json` Content-Type header.
An error entity has the following fields:

- **error**
  - **code**: The HTTP status code of the response
  - **message**: A human-readable error message explaining the failure

For example, if an invalid value is passed for a `nextPageToken`, the following HTTP response could be sent:

```
HTTP/1.1 400 Bad Request
Content-Type: application/json
Content-Length: 80

{"error:{"code":400,"message":"invalid value of nextPageToken query parameter"}}
```

[systemd-machine-id]: http://www.freedesktop.org/software/systemd/man/machine-id.html
[disco]: https://developers.google.com/discovery/v1/reference/apis
[schema]: /schema/v1.json
[example]: examples/api.py
