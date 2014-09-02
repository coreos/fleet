package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
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
	cmdUnloadUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are inactive, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdUnloadUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the units have become inactive before exiting.")
}

func runUnloadUnit(args []string) (exit int) {
	units, err := findUnits(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	wait := make([]string, 0)
	for _, s := range units {
		if !suToGlobal(s) {
			if job.JobState(s.CurrentState) == job.JobStateInactive {
				log.V(1).Infof("Target state of Unit(%s) already %s, skipping.", s.Name, job.JobStateInactive)
				continue
			}
		}

		log.V(1).Infof("Setting target state of Unit(%s) to %s", s.Name, job.JobStateInactive)
		cAPI.SetUnitTargetState(s.Name, string(job.JobStateInactive))
		if suToGlobal(s) {
			fmt.Printf("Triggered global unit %s unload\n", s.Name)
		} else {
			wait = append(wait, s.Name)
		}
	}

	if !sharedFlags.NoBlock {
		errchan := waitForUnitStates(wait, job.JobStateInactive, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			exit = 1
		}
	}

	return
}
