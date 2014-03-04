#!/bin/bash -e

source ./build

if [ -z "$PKG" ]; then
    PKG="./agent ./config ./job ./machine ./unit ./registry ./event ./sign ./integration-tests"
fi

# Unit tests
echo
go test -i $PKG
go test -cover -v $PKG
