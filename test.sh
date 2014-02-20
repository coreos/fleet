#!/bin/bash -e

source ./build

if [ -z "$PKG" ]; then
    PKG="./job ./machine ./unit ./registry ./event"
fi

# Unit tests
echo
go test -i $PKG
go test -cover -v $PKG
