package main

import (
	"fmt"
	"os"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

var cmdStopUnit = &Command{
	Name:    "stop",
	Summary: "Instruct systemd to stop one or more units in the cluster.",
	Usage:   "UNIT...",
	Description: `Stop one or more units from running in the cluster, but allow them to be
started again in the future.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Stop a single unit:
	fleetctl stop foo.service

Stop an entire directory of units with glob matching:
	fleetctl stop myservice/*`,
	Run: runStopUnit,
}

func init() {
	cmdStopUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are stopped, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdStopUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the units have stopped before exiting.")
}

func runStopUnit(args []string) (exit int) {
	units, err := findUnits(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	stopping := make([]string, 0)
	for _, u := range units {
		if job.JobState(u.CurrentState) == job.JobStateInactive {
			fmt.Fprintf(os.Stderr, "Unable to stop unit %s in state %s\n", u.Name, job.JobStateInactive)
			return 1
		} else if job.JobState(u.CurrentState) == job.JobStateLoaded {
			log.V(1).Infof("Unit(%s) already %s, skipping.", u.Name, job.JobStateLoaded)
			continue
		}

		log.V(1).Infof("Setting target state of Unit(%s) to %s", u.Name, job.JobStateLoaded)
		cAPI.SetUnitTargetState(u.Name, string(job.JobStateLoaded))
		stopping = append(stopping, u.Name)
	}

	if !sharedFlags.NoBlock {
		errchan := waitForUnitStates(stopping, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			exit = 1
		}
	}

	return
}
