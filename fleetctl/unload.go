package main

import (
	"fmt"
	"os"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

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
	cmdUnloadUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are inactive, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdUnloadUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the units have become inactive before exiting.")
}

func runUnloadUnit(args []string) (exit int) {
	sUnits, err := findScheduledUnits(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	wait := make([]string, 0)
	for _, su := range sUnits {
		if su.State == nil {
			fmt.Fprintf(os.Stderr, "Unable to determine state of unit %q\n", su.Name)
			return 1
		}

		if *(su.State) == job.JobStateInactive {
			log.V(1).Infof("Unit(%s) already %s, skipping.", su.Name, job.JobStateInactive)
			continue
		}

		log.V(1).Infof("Setting target state of Unit(%s) to %s", su.Name, job.JobStateInactive)
		cAPI.SetUnitTargetState(su.Name, job.JobStateInactive)
		wait = append(wait, su.Name)
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
