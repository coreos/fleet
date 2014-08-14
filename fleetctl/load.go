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
	cmdLoadUnits.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are loaded, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the units have been loaded before exiting.")
}

func runLoadUnits(args []string) (exit int) {
	if err := lazyCreateUnits(args, sharedFlags.Sign); err != nil {
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
