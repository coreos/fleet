#!/bin/bash -x

name=$1
if [ -z $name ]; then
	echo "Provide the name of the keys to remove"
	exit 1
fi

shift 1

for machine in $(fleetctl $@ list-machines --no-legend --full | awk '{ print $1;}'); do
	fleetctl $@ ssh $machine "update-ssh-keys -d $name -n"
done
