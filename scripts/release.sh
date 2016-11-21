#!/usr/bin/env bash
#
# Build all release binaries and images to directory ./release.
# Run from repository root.
#
set -e

VERSION=$1
if [ -z "${VERSION}" ]; then
	echo "Usage: ${0} VERSION" >> /dev/stderr
	exit 255
fi

if ! command -v acbuild >/dev/null; then
    echo "cannot find acbuild"
    exit 1
fi

if ! command -v docker >/dev/null; then
    echo "cannot find docker"
    exit 1
fi

FLEET_ROOT=$(dirname "${BASH_SOURCE}")/..

# build-aci is located in the top src directory until v0.13.0,
# while it's under ./scripts/ since v1.0.0.
BUILD_ACI="./scripts/build-aci"
if [ ! -x "${BUILD_ACI}" ]; then
	BUILD_ACI="./build-aci"
fi

pushd ${FLEET_ROOT} >/dev/null
	echo Building fleet binaries...
	./scripts/build-binary ${VERSION}
	echo Building aci image...
	BINARYDIR=release/fleet-${VERSION}-linux-amd64 BUILDDIR=release ${BUILD_ACI} ${VERSION}
	echo Building docker image...
	BINARYDIR=release/fleet-${VERSION}-linux-amd64 BUILDDIR=release ./scripts/build-docker ${VERSION}
popd >/dev/null
