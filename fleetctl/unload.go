package main

import (
	"fmt"
	"os"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

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

func init() {
	cmdUnloadUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the jobs are inactive, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdUnloadUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have become inactive before exiting.")
}

func runUnloadUnit(args []string) (exit int) {
	jobs, err := findJobs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	wait := make([]string, 0)
	for _, j := range jobs {
		if j.State == nil {
			fmt.Fprintf(os.Stderr, "Unable to determine state of %q\n", *(j.State))
			return 1
		}

		if *(j.State) == job.JobStateInactive {
			log.V(1).Infof("Job(%s) already %s, skipping.", j.Name, job.JobStateInactive)
			continue
		}

		log.V(1).Infof("Unloading Job(%s)", j.Name)
		registryCtl.SetJobTargetState(j.Name, job.JobStateInactive)
		wait = append(wait, j.Name)
	}

	if !sharedFlags.NoBlock {
		errchan := waitForJobStates(wait, job.JobStateInactive, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			exit = 1
		}
	}

	return
}
