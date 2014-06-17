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

## Machines

### Machine Entity

A Machine represents a host in the cluster.
It uses the host's [machine-id][systemd-machine-id] as a unique identifier.

[systemd-machine-id]: http://www.freedesktop.org/software/systemd/man/machine-id.html

- **id**: unique identifier of Machine entity
- **primaryIP**: IP address that should be used to communicate with this host
- **metadata**: dictionary of key-value data published by the machine

### List Machines

Explore a collection of Machine entities.

#### Request

```
GET /machines HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a list of zero or more Machine entities.
