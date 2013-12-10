#!/bin/sh
set -e

if [ -z "$PKG" ]; then
    PKG="./job ./machine ./unit"
fi

# Get GOPATH, etc from build
. ./build

# use right GOPATH
export GOPATH="${PWD}"

# Unit tests
for i in $PKG
do
    go test -i $i
    go test -v $i
done
