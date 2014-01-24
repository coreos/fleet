#!/bin/sh

if [ -z "$PKG" ]; then
    PKG="./job ./machine ./unit ./registry"
fi

# Unit tests
for i in $PKG
do
    go run third_party.go test -i $i
    go run third_party.go test -v $i
done
