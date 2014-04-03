#!/bin/bash -e

source ./build

if [ -z "$PKG" ]; then
	PKG="./agent ./config ./event ./fleetctl ./job ./machine ./pkg ./registry ./sign ./ssh ./unit ./integration-tests"
	GOFMTPATH="$PKG ./engine ./functional ./server ./systemd fleet.go"
else
	GOFMTPATH="$PKG"
fi

# Unit tests
echo
go test -i $PKG
go test -cover -v $PKG

fmtRes=`gofmt -l $GOFMTPATH`
if [ "$fmtRes" != "" ]; then
	echo "Failed to pass golang format checking:"
	echo $fmtRes
	exit 1
fi
