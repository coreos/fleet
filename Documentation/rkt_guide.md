# Running fleetd under rkt

The following guide will show you how to run fleetd under rkt.

## Building fleet ACI

Just run the [build-aci][build-aci] script.

## Running fleet ACI

You'll need a `/run/fleet/units` directory in the host since this is where fleet stores systemd units and it will be mounted as a volume inside the container.

Then you can run fleet under rkt:

```
# rkt --insecure-options=image run --inherit-env \
  --volume machine-id,kind=host,source=/etc/machine-id \
  --volume dbus-socket,kind=host,source=/run/dbus/system_bus_socket \
  --volume fleet-units,kind=host,source=/run/fleet/units \
  --volume etc-fleet,kind=host,source=/etc/fleet \
  fleetd-0.9.0.aci
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
ExecStart=/usr/bin/rkt --insecure-options=image run --inherit-env --volume machine-id,kind=host,source=/etc/machine-id --volume dbus-socket,kind=host,source=/run/dbus/system_bus_socket --volume fleet-units,kind=host,source=/run/fleet/units --volume etc-fleet,kind=host,source=/etc/fleet /usr/images/fleetd-0.9.0.aci
Restart=always
RestartSec=10s
```

[build-aci]: /build-aci
[deployment-and-configuration]: deployment-and-configuration.md
