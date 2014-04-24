package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/job"
)

var (
	flagRequire       string
	flagBlockAttempts int
	flagNoBlock       bool

	cmdStartUnit = &Command{
		Name:    "start",
		Summary: "Instruct systemd to start one or more units in the cluster, first submitting and loading if necessary.",
		Usage:   "[--sign] [--no-block|--block-attempts=N] UNIT...",
		Description: `Start one or many units on the cluster. Select units to start by glob matching
for units in the current working directory or matching names of previously
submitted units.

Start a single unit:
	fleetctl start foo.service

Start an entire directory of units with glob matching:
	fleetctl start myservice/*

You may filter suitable hosts based on metadata provided by the machine.
Machine metadata is located in the fleet configuration file.`,
		Run: runStartUnit,
	}
)

func init() {
	cmdStartUnit.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "Sign unit file signatures using local SSH identities.")
	cmdStartUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 10, "Wait until the jobs are launched, performing up to N attempts before giving up.")
	cmdStartUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have been launched before exiting.")
}

func runStartUnit(args []string) (exit int) {
	if err := lazyCreateJobs(args, sharedFlags.Sign); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	triggered, err := lazyStartJobs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if !sharedFlags.NoBlock {
		errchan := waitForJobStates(triggered, job.JobStateLaunched, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			exit = 1
		}
	} else {
		for _, jobName := range triggered {
			fmt.Printf("Triggered job %s start\n", jobName)
		}
	}

	return
}
