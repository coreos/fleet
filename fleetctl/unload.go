package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/job"
)

var (
	cmdUnloadUnit = &Command{
		Name:    "unload",
		Summary: "Unschedule one or more units in the cluster.",
		Usage:   "UNIT...",
		Run:     runUnloadUnit,
	}
)

func runUnloadUnit(args []string) (exit int) {
	jobs, err := findJobs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	for _, j := range jobs {
		if j.State == nil || *(j.State) != job.JobStateLoaded {
			fmt.Fprintf(os.Stderr, "Unable to unload job in state %q\n", *(j.State))
			return 1
		}

		registryCtl.SetJobTargetState(j.Name, "inactive")
	}

	return
}
