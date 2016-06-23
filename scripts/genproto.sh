#!/usr/bin/env bash
#
# Generate all fleet protobuf bindings.
# Run from repository root.
#
set -e

if ! [[ "$0" =~ "scripts/genproto.sh" ]]; then
	echo "must be run from repository root"
	exit 255
fi

# for now, be conservative about what version of protoc we expect
if ! [[ $(protoc --version) =~ "3.0.0" ]]; then
	echo "could not find protoc 3.0.0, is it installed + in PATH?"
	exit 255
fi

# directories containing protos to be built
DIRS="./protobuf"

# exact version of protoc-gen-gogo to build
GOGO_PROTO_SHA="2752d97bbd91927dd1c43296dbf8700e50e2708c"

# set up self-contained GOPATH for building
export GOPATH=${PWD}/gopath
export GOBIN=${PWD}/bin
export PATH="${GOBIN}:${PATH}"

COREOS_ROOT="${GOPATH}/src/github.com/coreos"
FLEET_ROOT="${COREOS_ROOT}/fleet"
GOGOPROTO_ROOT="${GOPATH}/src/github.com/gogo/protobuf"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"

rm -f "${FLEET_ROOT}"
mkdir -p "${COREOS_ROOT}"
ln -s "${PWD}" "${FLEET_ROOT}"

# Ensure we have the right version of protoc-gen-gogo by building it every time.
go get -u github.com/gogo/protobuf/{proto,protoc-gen-gogo,gogoproto}
go get -u golang.org/x/tools/cmd/goimports
pushd "${GOGOPROTO_ROOT}"
	git reset --hard "${GOGO_PROTO_SHA}"
	make install
popd

for dir in ${DIRS}; do
	[ ! -d "${dir}" ] && continue
	pushd ${dir}
		protoc --gogofast_out=plugins=grpc,import_prefix=github.com/coreos/:. --proto_path=.:"${GOGOPROTO_PATH}":"${COREOS_ROOT}":${GOPATH}/src *.proto
		sed -i.bak -E "s/github\.com\/coreos\/(gogoproto|github\.com|golang\.org|google\.golang\.org)/\1/g" *.pb.go
		sed -i.bak -E 's/github\.com\/coreos\/(errors|fmt|io)/\1/g' *.pb.go
		sed -i.bak -E 's/import _ \"gogoproto\"//g' *.pb.go
		sed -i.bak -E 's/import fmt \"fmt\"//g' *.pb.go
		rm -f *.bak
		goimports -w *.pb.go
	popd
done
