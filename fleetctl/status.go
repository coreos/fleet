package main

import (
	"fmt"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/schema"
)

var cmdStatusUnits = &Command{
	Name:    "status",
	Summary: "Output the status of one or more units in the cluster",
	Usage:   "UNIT...",
	Description: `Output the status of one or more units currently running in the cluster.
Supports glob matching of units in the current working directory or matches
previously started units.

Show status of a single unit:
	fleetctl status foo.service

Show status of an entire directory with glob matching:
fleetctl status myservice/*

This command does not work with global units.`,
	Run: runStatusUnits,
}

func runStatusUnits(args []string) (exit int) {
	units, err := cAPI.Units()
	if err != nil {
		stderr("Error retrieving unit: %v", err)
		return 1
	}

	uMap := make(map[string]*schema.Unit, len(args))
	for _, u := range units {
		if u != nil {
			u := u
			uMap[u.Name] = u
		}
	}

	names := make([]string, len(args))
	for i, arg := range args {
		name := unitNameMangle(arg)
		names[i] = name

		u, ok := uMap[name]
		if !ok {
			stderr("Unit %s does not exist.", name)
			return 1
		} else if suToGlobal(*u) {
			stderr("Unable to determine status of global unit %s.", name)
			return 1
		} else if job.JobState(u.CurrentState) == job.JobStateInactive {
			stderr("Unit %s does not appear to be loaded.", name)
			return 1
		}
	}

	for i, name := range names {
		// This extra newline is here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		cmd := fmt.Sprintf("systemctl status -l %s", name)
		if exit := runCommand(cmd, uMap[name].MachineID); exit != 0 {
			break
		}
	}
	return
}
