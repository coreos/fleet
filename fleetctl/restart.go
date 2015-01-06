/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"os"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/schema"
)

var cmdRestartUnit = &Command{
	Name:    "restart",
	Summary: "Instruct systemd to rolling restart one or more units in the cluster.",
	Usage:   "[--block-attempts=N] UNIT...",
	Description: `Restarts one or more units in the cluster. If they are stopped it starts them.

Instructs systemd on the host machine to stop then start the unit, deferring to systemd
completely for any custom restart directives (i.e. ExecStop options in the unit
file).

For units which are not global, restart operations are performed synchronously,
which means fleetctl will block until it detects that the unit(s) have
transitioned to a stopped state and then back to a started state. This behaviour can be configured with the
respective --block-attempts options. Restart operations on global
units are always non-blocking.

Restart a single unit:
	fleetctl restart foo.service

Restart an entire directory of units with glob matching, without waiting:
	fleetctl restart myservice/*`,
	Run: runRestartUnit,
}

func init() {
	cmdRestartUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are stopped/started, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units.")
}

func runRestartUnit(args []string) (exit int) {
	units, err := findUnits(args)
	if err != nil {
		stderr("%v", err)
		return 1
	}
	if len(units) == 0 {
		stderr("No units were found")
		return 1
	}
	for _, u := range units {
		if job.JobState(u.CurrentState) == job.JobStateInactive && !suToGlobal(u) {
			stderr("Unable to restart unit %s in state %s", u.Name, job.JobStateInactive)
			return 1
		} else if job.JobState(u.CurrentState) == job.JobStateLoaded {
			log.V(1).Infof("Unit(%s) already %s, starting.", u.Name, job.JobStateLoaded)

			exit = setUnitStateAndWait(u, job.JobStateLaunched, sharedFlags.BlockAttempts)
			if exit == 1 {
				return exit
			}

			continue
		} else {
			//stop and start it
			exit = setUnitStateAndWait(u, job.JobStateLoaded, sharedFlags.BlockAttempts)
			if exit == 1 {
				return exit
			}
			exit = setUnitStateAndWait(u, job.JobStateLaunched, sharedFlags.BlockAttempts)
			if exit == 1 {
				return exit
			}
		}

		stdout("Unit(%s) was restarted.", u.Name)
	}

	return
}

func setUnitStateAndWait(unit schema.Unit, targetState job.JobState, blockAttempts int) (exit int) {
	cAPI.SetUnitTargetState(unit.Name, string(targetState))

	if !suToGlobal(unit) {
		unitArr := make([]string, 1)
		unitArr[0] = unit.Name
		errchan := waitForUnitStates(unitArr, targetState, blockAttempts, os.Stdout)
		for err := range errchan {
			stderr("Error waiting for unit %s to change states: %v", unit.Name, err)
			return 1
		}
	}
	return
}
