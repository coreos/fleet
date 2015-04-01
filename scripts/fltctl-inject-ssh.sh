#!/bin/bash -x

name=$1
if [ -z $name ]; then
	echo "Provide a name for the injected SSH key"
	exit 1
fi

shift 1

pubkey=$(cat)

for machine in $(fltctl $@ list-machines --no-legend --full | awk '{ print $1;}'); do
	fltctl $@ ssh $machine "echo '${pubkey}' | update-ssh-keys -a $name -n"
done
