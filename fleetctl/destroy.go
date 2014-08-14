package main

import (
	"fmt"
)

var cmdDestroyUnit = &Command{
	Name:    "destroy",
	Summary: "Destroy one or more units in the cluster",
	Usage:   "UNIT...",
	Description: `Completely remove one or more running or submitted units from the cluster.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Destroyed units are impossible to start unless re-submitted.`,
	Run: runDestroyUnits,
}

func runDestroyUnits(args []string) (exit int) {
	for _, v := range args {
		name := unitNameMangle(v)
		err := cAPI.DestroyUnit(name)
		if err != nil {
			continue
		}

		fmt.Printf("Destroyed %s\n", name)
	}
	return
}
