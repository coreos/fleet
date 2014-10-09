package main

import (
	"os"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
)

var cmdRestartUnit = &Command{
	Name:    "restart",
	Summary: "Instruct systemd to restart one or more units in the cluster.",
	Usage:   "[--no-block|--block-attempts=N] UNIT...",
	Description: `Restart one or more units running in the cluster.

Instructs systemd on the host machine to restart the unit, deferring to systemd
completely for any custom start and stop directives (i.e. ExecStop or ExecStart
options in the unit file).

For units which are not global, restart operations are performed synchronously,
which means fleetctl will block until it detects that the unit(s) have
transitioned to a stopped state and then back to a start state. This behaviour
can be configured with the respective --block-attempts and --no-block options.
Restart operations on global units are always non-blocking.

Restart a single unit:
  fleetctl restart foo.service

Restart an entire directory of units with glob matching, without waiting:
  fleetctl --no-block restart myservice/*`,
	Run: runRestartUnit,
}

func init() {
	cmdRestartUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are stopped and then started again, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units.")
	cmdRestartUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the units have restarted before exiting. Always the case for global units.")
}

func runRestartUnit(args []string) (exit int) {
	units, err := findUnits(args)
	if err != nil {
		stderr("%v", err)
		return 1
	}

	stopping := make([]string, 0)
	for _, u := range units {
		if !suToGlobal(u) {
			if job.JobState(u.CurrentState) == job.JobStateInactive {
				stderr("Unable to stop unit %s in state %s", u.Name, job.JobStateInactive)
				return 1
			} else if job.JobState(u.CurrentState) == job.JobStateLoaded {
				log.V(1).Infof("Unit(%s) already %s, skipping.", u.Name, job.JobStateLoaded)
				continue
			}
		}

		log.V(1).Infof("Setting target state of Unit(%s) to %s", u.Name, job.JobStateLoaded)
		cAPI.SetUnitTargetState(u.Name, string(job.JobStateLoaded))
		if suToGlobal(u) {
			stdout("Triggered global unit %s stop", u.Name)
		} else {
			stopping = append(stopping, u.Name)
		}

		if !sharedFlags.NoBlock {
			errchan := waitForUnitStates([](string){u.Name}, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
			for err := range errchan {
				stderr("Error waiting for units: %v", err)
				exit = 1
			}
		} else {
			for _, name := range stopping {
				stdout("Triggered unit %s stop", name)
			}
		}

		var starting []string
		cAPI.SetUnitTargetState(u.Name, string(job.JobStateLaunched))

		if suToGlobal(u) {
			stdout("Triggered global unit %s start", u.Name)
		} else {
			starting = append(starting, u.Name)
		}

		if !sharedFlags.NoBlock {
			errchan := waitForUnitStates([](string){u.Name}, job.JobStateLaunched, sharedFlags.BlockAttempts, os.Stdout)
			for err := range errchan {
				stderr("Error waiting for units: %v", err)
				exit = 1
			}
		} else {
			for _, name := range starting {
				stdout("Triggered unit %s start", name)
			}
		}
	}

	return
}
