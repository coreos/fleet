package main

import (
	"os"

	"github.com/coreos/fleet/job"
)

var cmdOverwriteUnit = &Command{
	Name:    "overwrite",
	Summary: "Overwrite one or more units in the cluster",
	Usage:   "UNIT...",
	Description: `Overwrite one or more running or submitted units from the cluster.

Act as a procedure of stop-->destroy-->commit-->start(if needed) on unit(s)`,

	Run: runOverwriteUnits,
}

func runOverwriteUnits(args []string) (exit int) {
	for _, v := range args {
		v = maybeAppendDefaultUnitType(v)
		name := unitNameMangle(v)

		u, err := cAPI.Unit(name)
		if err != nil {
			stderr("error retrieving Unit(%s) from Registry: %v", name, err)
			return 1
		}
		if u == nil {
			stdout("Unit(%s) in can not be found in Registry, nothing to do with it", name)
			continue
		}

		hash_mismatch := warnOnDifferentLocalUnit(v, u)
		if hash_mismatch == false {
			stdout("Nothing different between Unit(%s) in registory and local unit file, just skip", name)
			continue
		}

		stopping := make([]string, 0)
		stopping = append(stopping, u.Name)
		if job.JobState(u.CurrentState) == job.JobStateLaunched || suToGlobal(*u) {
			stdout("Stop Unit(%s) first, so that we can overwrite unit file", u.Name)
			cAPI.SetUnitTargetState(u.Name, string(job.JobStateLoaded))
			errchan := waitForUnitStates(stopping, job.JobStateLoaded, 0, os.Stdout)
			for err := range errchan {
				stderr("Error waiting for units: %v", err)
				exit = 1
			}
		}

		err = cAPI.DestroyUnit(name)
		if err != nil {
			continue
		}

		recreating := make([]string, 0)
		recreating = append(recreating, v)
		stdout("Recreating unit(%s)", v)
		if err := lazyCreateUnits(recreating); err != nil {
			stderr("Error creating units: %v", err)
			return
		}
		if job.JobState(u.CurrentState) == job.JobStateInactive {
			continue
		} else if job.JobState(u.CurrentState) == job.JobStateLoaded {
			stdout("Rescheduling unit(%s)", v)
			triggered, err := lazyLoadUnits(recreating)
			if err != nil {
				stderr("Error loading units: %v", err)
				return 1
			}

			var loading []string
			for _, u := range triggered {
				if suToGlobal(*u) {
					stdout("Triggered global unit %s load", u.Name)
				} else {
					loading = append(loading, u.Name)
				}
			}

			errchan := waitForUnitStates(loading, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
			for err := range errchan {
				stderr("Error waiting for units: %v", err)
			}
		} else if job.JobState(u.CurrentState) == job.JobStateLaunched {
			stdout("Restarting unit(%s)", v)
			triggered, err := lazyStartUnits(recreating)
			if err != nil {
				stderr("Error starting units: %v", err)
				return 1
			}

			var starting []string
			for _, u := range triggered {
				if suToGlobal(*u) {
					stdout("Triggered global unit %s start", u.Name)
				} else {
					starting = append(starting, u.Name)
				}
			}

			errchan := waitForUnitStates(starting, job.JobStateLaunched, sharedFlags.BlockAttempts, os.Stdout)
			for err := range errchan {
				stderr("Error waiting for units: %v", err)
				exit = 1
			}
		}

	}
	return
}
