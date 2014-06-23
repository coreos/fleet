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
- **desiredState**: state the user wishes the Unit to be in (inactive, loaded, or launched)
- **fileContents**: base64-encoded contents of the Unit's file
- **fileHash**: SHA1 hash of the Unit's file contents
- **currentState**: last known state of the Unit (inactive, loaded, or launched)
- **targetMachineID**: identifier of the Machine to which this Unit is currently scheduled
- **sytemd**:
  - **loadState**: LOAD state of the underlying systemd unit
  - **activeState**: ACTIVE state of the underlying systemd unit
  - **subState**: SUB state of the underlying systemd unit
  - **machineID**: identifier of the Machine that published this systemd state


### Create & Modify Units

```
PUT /units/<name> HTTP/1.1
```

#### Request

A request is comprised of a single partiall Unit entity.
If creating a new Unit, supply the name, desiredState and fileContents fields.
To modify an existing Unit, provide just the name and desiredState.
The base datastructure looks like so:

```
{"desiredState": <state>, "fileContents": <encoded-contents>}
```

For example, launching a new unit "foo.service" could look like this:

```
{
  "desiredState": "launched",
  "fileContents": "W1NlcnZpY2VdCkV4ZWNTdGFydD0vdXNyL2Jpbi9zbGVlcCAzMDAwCg=="
}
```

#### Response

A successful response contains no body.

### List Units

Explore a paginated collection of Unit entities.

#### Request

```
GET /units HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a single page of zero or more Unit entities.

### Fetch Unit

Retrieve a single Unit entity.

#### Request

```
GET /units/<name> HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a single Unit entity.
If the indicated Unit does not exist, a `404 Not Found` will be returned.

### Destroy Units

Destroy one or more existing Unit entities.

#### Request

```
DELETE /units/<name> HTTP/1.1
```

Indicate which Units should be destroyed in the URL.

#### Response

A successful response will not contain a body or any additional headers.
If the indicated Unit does not exist, a `404 Not Found` will be returned.

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
