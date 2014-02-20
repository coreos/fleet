#!/bin/bash -e

source ./build

if [ -z "$PKG" ]; then
    PKG="./job ./machine ./unit ./registry ./event"
fi

# Unit tests
for i in $PKG
do
    echo
    echo "Testing $i"
    go test -i $i
    go test -cover -v $i
done
