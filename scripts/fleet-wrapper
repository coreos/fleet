#!/bin/bash
# Wrapper for launching fleet via rkt-fly.
#
# Users could set FLEET_IMAGE_TAG to an image tag published here:
# https://quay.io/repository/coreos/fleet?tab=tags Alternatively,
# override FLEET_IMAGE to a custom image.

RKT_GLOBAL_ARGS="--insecure-options=image"

FLEET_IMAGE_URL="${FLEET_IMAGE_URL:-quay.io/coreos/fleet}"
FLEET_IMAGE_TAG="${FLEET_IMAGE_TAG:-v1.0.0}"
FLEET_IMAGE="${FLEET_IMAGE:-${FLEET_IMAGE_URL}:${FLEET_IMAGE_TAG}}"
FLEET_USER="${FLEET_USER:-fleet}"

if [[ "${FLEET_IMAGE%%/*}" == "quay.io" ]]; then
	RKT_RUN_ARGS="${RKT_RUN_ARGS} --trust-keys-from-https"
fi

mkdir --parents /etc/fleet
mkdir --parents /run/dbus
mkdir --parents /run/fleet

RKT="${RKT:-/usr/bin/rkt}"
RKT_STAGE1_ARG="${RKT_STAGE1_ARG:---stage1-path=/usr/lib/rkt/stage1-images/stage1-fly.aci}"
set -x
exec ${RKT} ${RKT_GLOBAL_ARGS} \
	${RKT_STAGE1_ARG} \
	run ${RKT_RUN_ARGS} \
	--volume etc-fleet,kind=host,source=/etc/fleet,readOnly=true \
	--volume machine-id,kind=host,source=/etc/machine-id,readOnly=true \
	--volume run,kind=host,source=/run,readOnly=false \
	--mount volume=etc-fleet,target=/etc/fleet \
	--mount volume=machine-id,target=/etc/machine-id \
	--mount volume=run,target=/run \
	--inherit-env \
	--set-env=DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket \
	${FLEET_IMAGE} \
	--user=$(id -u "${FLEET_USER}") \
	-- "$@"
