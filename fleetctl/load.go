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
		Usage:   "[--no-block|--block-attempts=N] UNIT...",
		Run:     runLoadUnits,
	}
)

func init() {
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "DEPRECATED - this option cannot be used")
	cmdLoadUnits.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the jobs are loaded, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have been loaded before exiting.")
}

func runLoadUnits(args []string) (exit int) {
	if err := lazyCreateUnits(args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	triggered, err := lazyLoadUnits(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if !sharedFlags.NoBlock {
		errchan := waitForUnitStates(triggered, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			exit = 1
		}
	} else {
		for _, name := range triggered {
			fmt.Printf("Triggered unit %s load\n", name)
		}
	}

	return
}
