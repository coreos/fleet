package main

import (
	"fmt"
	"os"
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

func printUnitStatus(jobName string) int {
	j := registryCtl.GetJob(jobName)
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not exist.\n", jobName)
		os.Exit(1)
	} else if j.PayloadState == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not appear to be running.\n", jobName)
		return 1
	}

	cmd := fmt.Sprintf("systemctl status -l %s", jobName)
	retcode, err := runCommand(cmd, j.PayloadState.MachineState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed running command over SSH: %v\n", err)
		return 1
	}

	return retcode
}
