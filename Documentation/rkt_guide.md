# Running fleetd under rkt

**[fleet is no longer actively developed or maintained by CoreOS](https://coreos.com/blog/migrating-from-fleet-to-kubernetes.html). CoreOS instead recommends [Kubernetes](https://coreos.com/kubernetes/docs/latest/) for cluster orchestration.**

The following guide will show you how to run fleetd under rkt.

## Building fleet ACI

Just run the [build-aci][build-aci] script.

## Running fleet ACI

You'll need a `/run/fleet/units` directory in the host since this is where fleet stores systemd units and it will be mounted as a volume inside the container.

Then you can run fleet under rkt:

```
# rkt --insecure-options=image run --inherit-env \
  --volume etc-fleet,kind=host,source=/etc/fleet \
  --volume machine-id,kind=host,source=/etc/machine-id \
  --volume dbus-socket,kind=host,source=/run/dbus/system_bus_socket \
  --volume fleet-units,kind=host,source=/run/fleet/units \
  --mount volume=etc-fleet,target=/etc/fleet \
  --mount volume=machine-id,target=/etc/machine-id \
  --mount volume=dbus-socket,target=/run/dbus/system_bus_socket \
  --mount volume=fleet-units,target=/run/fleet/units \
  fleetd-0.13.0.aci
```

You can configure it modifying the file `/etc/fleet/fleet.conf` in your host and with enviroment variables as described in [deployment-and-configuration.md][deployment-and-configuration].

## Example systemd unit file

```
[Unit]
Description=fleet daemon (rkt flavor)

Wants=etcd.service
After=etcd.service

Wants=fleet.socket
After=fleet.socket

[Service]
ExecStartPre=/usr/bin/mkdir -p /run/fleet/units
ExecStart=/usr/bin/rkt --insecure-options=image run --inherit-env --volume etc-fleet,kind=host,source=/etc/fleet --volume machine-id,kind=host,source=/etc/machine-id --volume dbus-socket,kind=host,source=/run/dbus/system_bus_socket --volume fleet-units,kind=host,source=/run/fleet/units --mount volume=etc-fleet,target=/etc/fleet --mount volume=machine-id,target=/etc/machine-id --mount volume=dbus-socket,target=/run/dbus/system_bus_socket --mount volume=fleet-units,target=/run/fleet/units /usr/images/fleetd-0.13.0.aci
Restart=always
RestartSec=10s
```

[build-aci]: ../scripts/build-aci
[deployment-and-configuration]: deployment-and-configuration.md
