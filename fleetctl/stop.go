package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/job"
)

var cmdStopUnit = &Command{
	Name:    "stop",
	Summary: "Instruct systemd to stop one or more units in the cluster.",
	Usage:   "UNIT...",
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
	jobs, err := findJobs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	for _, j := range jobs {
		if j.State == nil || *(j.State) != job.JobStateLaunched {
			fmt.Fprintf(os.Stderr, "Unable to stop job in state %q\n", *(j.State))
			return 1
		}

		registryCtl.SetJobTargetState(j.Name, "loaded")
	}
	return
}
