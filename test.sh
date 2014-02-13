#!/bin/bash -e

source ./build

if [ -z "$PKG" ]; then
    PKG="./job ./machine ./unit ./registry"
fi

# Unit tests
for i in $PKG
do
    go test -i $i
    go test -v $i
done
