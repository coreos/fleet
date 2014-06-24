# fleet API v1-alpha (EXPERIMENTAL)

The API this document describes is not yet finalized, so clients should expect it to change.
This document describes only what has been implemented, not the final state
The version of the API will transition from "v1-alpha" to "v1" when it has been finalized, and the EXPERIMENTAL label will be removed.

## Capability Discovery

The v1 fleet API is described by a [discovery document][disco].
This document is available in the [fleet source][schema].

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


### Create or Modify Unit

```
PUT /units/<name> HTTP/1.1
```

#### Request

A request is comprised of a partial Unit entity.
If creating a new Unit, supply the desiredState and fileContents fields.
To modify an existing Unit, only the desiredState field is required.
If the fileContents field is provided in a modification request, the server will ensure the contents match the existing unit before making any changes.

The base datastructure looks like this:

```
{"desiredState": <state>, "fileContents": <encoded-contents>}
```

For example, launching a new unit "foo.service" could be done like so: 

```
PUT /units/foo.service HTTP/1.1

{
  "desiredState": "launched",
  "fileContents": "W1NlcnZpY2VdCkV4ZWNTdGFydD0vdXNyL2Jpbi9zbGVlcCAzMDAwCg=="
}
```

Unloading an existing Unit called "bar.service" could look like this:

```
PUT /units/bar.service HTTP/1.1

{
  "desiredState": "inactive"
}
```

The expected contents of "bar.service" could also be provided to make changes safely:

```
PUT /units/bar.service HTTP/1.1

{
  "desiredState": "inactive",
  "fileContents": "W1NlcnZpY2VdDQpFeGVjU3RhcnQ9L3Vzci9iaW4vc2xlZXAgMWQNCg=="
}
```

#### Response

A successful response contains no body.
Conflicts between fileContents values are indicated with a `409 Conflict` response.
Attempting to create an entity without fileContents will also return a `409 Conflict` response.

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

Destroy an existing Unit entity.

#### Request

```
DELETE /units/<name> HTTP/1.1
```

The provided request body may contain a single optional field: "fileContents".
If the fileContents field is provided, the server will ensure the contents match the existing unit before making any changes.

```
{"fileContents": <encoded-contents>}
```

#### Response

A successful response will not contain a body or any additional headers.
If the indicated Unit does not exist, a `404 Not Found` will be returned.
Conflicts between fileContents values are indicated with a `409 Conflict` response.

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
