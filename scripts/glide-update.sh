#!/usr/bin/env bash
#
# Update vendored dedendencies.
#
set -e

if ! [[ "$PWD" = "$GOPATH/src/github.com/coreos/fleet" ]]; then
	echo "must be run from \$GOPATH/src/github.com/coreos/fleet"
	exit 255
fi

if [ -z "$(command -v glide)" ]; then
	echo "glide: command not found"
	exit 255
fi

if [ -z "$(command -v glide-vc)" ]; then
	echo "glide-vc: command not found"
	exit 255
fi

glide update --strip-vcs --strip-vendor --update-vendored --delete
glide vc --only-code --no-tests --no-legal-files

# manual cleanup to reduce size of vendor trees

# etcd
ETCD_CLEANUP_DIRS="\
 alarm auth clientv3 cmd compactor contrib discovery e2e embed etcdctl \
 etcdmain etcdserver hack integration lease logos mvcc proxy raft rafthttp \
 scripts snap store tools version wal \
"
pushd vendor/github.com/coreos/etcd
	rm -rf $ETCD_CLEANUP_DIRS
popd

# godbus
rm -rf vendor/github.com/godbus/dbus/_examples

# go-systemd
GOSYSTEMD_CLEANUP_DIRS="examples journal login1 machine1 sdjournal util"
pushd vendor/github.com/coreos/go-systemd
	rm -rf $GOSYSTEMD_CLEANUP_DIRS
popd

# grpc
GRPC_CLEANUP_DIRS="benchmark examples test"
pushd vendor/google.golang.org/grpc
	rm -rf $GRPC_CLEANUP_DIRS
popd

# gogo/protobuf
GOGOPROTOBUF_CLEANUP_DIRS="test vanity/test"
pushd vendor/github.com/gogo/protobuf
	rm -rf $GOGOPROTOBUF_CLEANUP_DIRS
popd

