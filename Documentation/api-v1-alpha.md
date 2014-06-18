# fleet API v1-alpha (EXPERIMENTAL)

The API this document describes is not yet finalized, so clients should expect it to change.
This document describes only what has been implemented, not the final state
The version of the API will transition from "v1-alpha" to "v1" when it has been finalized, and the EXPERIMENTAL label will be removed.

## Capability Discovery

The v1 fleet API is described by a [discovery document][disco].
This document is available in the [fleet source][schema]:

[disco]: https://developers.google.com/discovery/v1/reference/apis
[schema]: ../schema/v1-alpha.json

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
GET /cats HTTP/1.1


HTTP/1.1 200 OK

{"cats": [{"id": "baxter"}, {"id": "charlie"}], "nextPageToken": "8fefec2c"}
```

```
GET /cats?nextPageToken=8fefec2c HTTP/1.1


HTTP/1.1 200 OK

{"cats": [{"id": "michael"}, {"id": "prescott"}], "nextPageToken": "cbb06916"}
```

```
GET /cats?nextPageToken=cbb06916 HTTP/1.1


HTTP/1.1 200 OK

{"cats": [{"id":"timothy"}]}
```
## Units

### Unit Entity

- **name**: unique identifier of entity
- **fileContents**: base64-encoded contents of the Unit's file
- **fileHash**: SHA1 hash of the Unit's file contents
- **desiredState**: state the user wishes the Unit to be in (inactive, loaded, or launched)
- **currentState**: last known state of the Unit (inactive, loaded, or launched)
- **loadState**: LOAD state of the underlying systemd unit
- **activeState**: ACTIVE state of the underlying systemd unit
- **subState**: SUB state of the underlying systemd unit
- **targetMachine**: identifier of the Machine to which this Unit is scheduled

### List Units

Explore a paginated collection of Unit entities.

#### Request

```
GET /units HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a single page of zero or more Unit entities.

### Destroy Units

Destroy one or more existing Unit entities.

#### Request

```
DELETE /units HTTP/1.1
```

Indicate which Units should be destroyed in the request body like so:

```
{"units": [{"name": <name>}, ... ]}
```

#### Response

A successful response will not contain a body or any additional headers.
If any Units entities do not exist, a `400 Bad Request` will be returned.

## Machines

### Machine Entity

A Machine represents a host in the cluster.
It uses the host's [machine-id][systemd-machine-id] as a unique identifier.

[systemd-machine-id]: http://www.freedesktop.org/software/systemd/man/machine-id.html

- **id**: unique identifier of Machine entity
- **primaryIP**: IP address that should be used to communicate with this host
- **metadata**: dictionary of key-value data published by the machine

### List Machines

Explore a paginated collection of Machine entities.

#### Request

```
GET /machines HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a page of zero or more Machine entities.
