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
	cmdUnloadUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 10, "Wait until the jobs are inactive, performing up to N attempts before giving up.")
	cmdUnloadUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have become inactive before exiting.")
}

func runUnloadUnit(args []string) (exit int) {
	jobs, err := findJobs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	stopping := make([]string, 0)
	unload := make([]string, 0)
	for _, j := range jobs {
		if j.State == nil {
			fmt.Fprintf(os.Stderr, "Unable to determine state of %q\n", *(j.State))
			return 1
		}

		if *(j.State) == job.JobStateInactive {
			log.V(1).Infof("Job(%s) already %s, skipping.", j.Name, job.JobStateInactive)
			continue
		} else if *(j.State) == job.JobStateLaunched {
			log.V(1).Infof("Stopping Job(%s) before unloading", j.Name)
			registryCtl.SetJobTargetState(j.Name, job.JobStateLoaded)
			stopping = append(stopping, j.Name)
		}

		unload = append(unload, j.Name)
	}

	// Always wait for jobs that had to be stopped regardless of the --no-block flag
	err = waitForJobStates(stopping, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	for _, jobName := range unload {
		log.V(1).Infof("Unloading Job(%s)", jobName)
		registryCtl.SetJobTargetState(jobName, job.JobStateInactive)
	}

	if !sharedFlags.NoBlock {
		if err := waitForJobStates(unload, job.JobStateInactive, sharedFlags.BlockAttempts, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	}

	return
}
