#!/bin/bash -x

name=$1
if [ -z $name ]; then
	echo "Provide a name for the injected SSH key"
	exit 1
fi

shift 1

pubkey=$(cat)

for machine in $(corectl $@ list-machines --no-legend | awk '{ print $1;}'); do
	corectl $@ ssh $machine "echo '${pubkey}' | update-ssh-keys -a $name -n"
done
