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
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "Sign unit file signatures and verify submitted units using local SSH identities.")
	cmdLoadUnits.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 10, "Wait until the jobs are loaded, performing up to N attempts before giving up.")
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have been loaded before exiting.")
}

func runLoadUnits(args []string) (exit int) {
	if err := lazyCreateJobs(args, sharedFlags.Sign); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	triggered, err := lazyLoadJobs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if !sharedFlags.NoBlock {
		err = waitForJobStates(triggered, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	} else {
		for _, jobName := range triggered {
			fmt.Printf("Triggered job %s load\n", jobName)
		}
	}

	return
}
