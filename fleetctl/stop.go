package main

import (
	"fmt"
	"path"
)

var cmdStopUnit = &Command{
	Name:    "stop",
	Summary: "Halt one or more units in the cluster",
	Description: `Stop one or more units from running in the cluster, but allow them to be
started again in the future.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Stop a single unit:
fleetctl stop foo.service

Stop an entire directory of units with glob matching:
fleetctl stop myservice/*`,
	Run: runStopUnit,
}

func runStopUnit(args []string) (exit int) {
	for _, v := range args {
		name := path.Base(v)
		registryCtl.StopJob(name)
		fmt.Printf("Requested Job %s stop\n", name)
	}
	return 0
}
