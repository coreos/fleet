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
glide vc --only-code --no-tests
