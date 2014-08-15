package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/job"
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
fleetctl status myservice/*`,
	Run: runStatusUnits,
}

func runStatusUnits(args []string) (exit int) {
	for i, v := range args {
		// This extra newline here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		name := unitNameMangle(v)
		exit = printUnitStatus(name)
		if exit != 0 {
			break
		}
	}
	return
}

func printUnitStatus(name string) int {
	u, err := cAPI.Unit(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving unit %s: %v", name, err)
		return 1
	}
	if u == nil {
		fmt.Fprintf(os.Stderr, "Unit %s does not exist.\n", name)
		return 1
	} else if job.JobState(u.CurrentState) == job.JobStateInactive {
		fmt.Fprintf(os.Stderr, "Unit %s does not appear to be running.\n", name)
		return 1
	}

	cmd := fmt.Sprintf("systemctl status -l %s", name)
	return runCommand(cmd, u.MachineID)
}
