package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/job"
)

var (
	cmdLoadUnits = &Command{
		Name:    "load",
		Summary: "Schedule one or more units in the cluster, first submitting them if necessary.",
		Usage:   "[--sign] [--no-block|--block-attempts=N] UNIT...",
		Run:     runLoadUnits,
	}
)

func init() {
	cmdLoadUnits.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the jobs are loaded, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have been loaded before exiting.")
}

func runLoadUnits(args []string) (exit int) {
	if err := lazyCreateJobs(args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	triggered, err := lazyLoadJobs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if !sharedFlags.NoBlock {
		errchan := waitForJobStates(triggered, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			exit = 1
		}
	} else {
		for _, jobName := range triggered {
			fmt.Printf("Triggered job %s load\n", jobName)
		}
	}

	return
}
