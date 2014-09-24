# fleet API v1-alpha (EXPERIMENTAL)

The API this document describes is not yet finalized, so clients should expect it to change.
This document describes only what has been implemented, not the final state.
The version of the API will transition from "v1-alpha" to "v1" when it has been finalized, and the EXPERIMENTAL label will be removed.

## Enabling the API

To enable the API, all you need to do is create a socket and start it manually or with cloud-config.

### Create the Socket Manually

#### /etc/systemd/system/fleet.socket
```
[Socket]
# Talk to the API over a Unix domain socket (default)
ListenStream=/var/run/fleet.sock
# Talk to the API over an exposed port, uncomment to enable and choose a port
#ListenStream=
Service=fleet.service
 
[Install]
WantedBy=sockets.target
 ```

To allow fleet to detect the new socket, enable it, then stop and start the fleet unit:

```sh
$ sudo systemctl enable /etc/systemd/system/fleet.socket
$ sudo systemctl stop fleet
$ sudo systemctl start fleet.socket
$ sudo systemctl start fleet
```

### Cloud-Config

To enable the API on your entire cluster, you can set up the socket in cloud-config:

```yaml
#cloud-config

coreos:
  etcd:
    # generate a new token for each unique cluster from https://discovery.etcd.io/new
    discovery: https://discovery.etcd.io/<token>
    # multi-region and multi-cloud deployments need to use $public_ipv4
    addr: $private_ipv4:4001
    peer-addr: $private_ipv4:7001
  units:
    - name: etcd.service
      command: start
    - name: fleet.socket
      command: start
      content: |
        [Socket]
        # Talk to the API over a Unix domain socket (default)
        ListenStream=/var/run/fleet.sock
        # Talk to the API over an exposed port, uncomment to enable and choose a port
        #ListenStream=
        Service=fleet.service
         
        [Install]
        WantedBy=sockets.target
    - name: fleet.service
      command: start
```

## Capability Discovery

The v1 fleet API is described by a [discovery document][disco]. Users should generate their client bindings from this document using the appropriate language generator.
This document is available in the [fleet source][schema] and served directly from the API itself, at the `/discovery.json` endpoint.
Note that this discovery document intentionally ships with an unusable `rootUrl`; clients *must* initialize this as appropriate.

An extremely simplified example client can be found [here][example].

[disco]: https://developers.google.com/discovery/v1/reference/apis
[schema]: ../schema/v1-alpha.json
[example]: ../Documentation/examples/api.py

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

## Managing Units

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

```
PUT /units/<name> HTTP/1.1
```

#### Request

Create a Unit by passing a partial Unit entity to the /units resource.
The options and desiredState fields are required, and all other Unit fields will be ignored.

The base datastructure looks like this:

```
{"desiredState": <state>, "options": [<option>, ...]}
```

For example, creating and launching a new unit "foo.service" could be done like so:

```
PUT /units/foo.service HTTP/1.1

{
  "desiredState": "launched",
  "options": [{"section": "Service", "name": "ExecStart", "value": "/usr/bin/sleep 3000"}]
}
```

**Note:** If the unit's name field is set in the request body, it must match the
name in the PUT /units/<name> request.

#### Response

A success is indicated by a `201 Created` but contains no body.
Attempting to create an entity without options will return a `409 Conflict` response.
Attempting to create an invalid entity will return a `400 Bad Request` response.

### Modify desired state of a Unit

```
PUT /units/<name> HTTP/1.1
```

#### Request

Modify the desired state by providing a partial Unit entity.
The only field used from this Unit entity is the desiredState.

The base datastructure looks like this:

```
{"desiredState": <state>}
```

For example, unloading an existing Unit called "bar.service" could look like this:

```
PUT /units/bar.service HTTP/1.1

{
  "desiredState": "inactive"
}
```

**Note:** If the unit's name field is set in the request body, it must match the
name in the PUT /units/<name> request.

#### Response

A success is indicated by a `204 No Content`.
Attempting to modify with an invalid entity will return a `400 Bad Request` response.

### Retrieve desired state of all Units

Explore a paginated collection of Unit entities.

#### Request

```
GET /units HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a single page of zero or more Unit entities.

### Retrieve desired state of a specific Unit

View a particular Unit entity.

#### Request

```
GET /units/<name> HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will contain a single Unit entity.
If the requested Unit does not exist, a `404 Not Found` will be returned.

### Destroy a Unit

Destroy the desired state of an existing Unit entity.

#### Request

```
DELETE /units/<name> HTTP/1.1
```

The request must not have a body.

#### Response

A successful response will not contain a body or any additional headers.
If the indicated Unit does not exist, a `404 Not Found` will be returned.

## Current Unit State

### UnitState Entity

- **name**: unique identifier of entity
- **hash**: SHA1 hash of underlying unit file
- **machineID**: ID of machine from which this state originated
- **systemdLoadState**: load state as reported by systemd
- **systemdActiveState**: active state as reported by systemd
- **systemdSubState**: sub state as reported by systemd

### Retrieve current state of all Units

Explore a paginated collection of UnitState entities.

#### Request

```
GET /state HTTP/1.1
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
