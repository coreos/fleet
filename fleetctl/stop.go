// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
)

var cmdStopUnit = &Command{
	Name:    "stop",
	Summary: "Instruct systemd to stop one or more units in the cluster.",
	Usage:   "[--no-block|--block-attempts=N] UNIT...",
	Description: `Stop one or more units from running in the cluster, but allow them to be
started again in the future.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

For units which are not global, stop operations are performed synchronously,
which means fleetctl will block until it detects that the unit(s) have
transitioned to a stopped state. This behaviour can be configured with the
respective --block-attempts and --no-block options. Stop operations on global
units are always non-blocking.

Stop a single unit:
	fleetctl stop foo.service

Stop an entire directory of units with glob matching, without waiting:
	fleetctl --no-block stop myservice/*`,
	Run: runStopUnit,
}

func init() {
	cmdStopUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are stopped, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units.")
	cmdStopUnit.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the units have stopped before exiting. Always the case for global units.")
}

func runStopUnit(args []string) (exit int) {
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
				log.Debugf("Unit(%s) already %s, skipping.", u.Name, job.JobStateLoaded)
				continue
			}
		}

		log.Debugf("Setting target state of Unit(%s) to %s", u.Name, job.JobStateLoaded)
		cAPI.SetUnitTargetState(u.Name, string(job.JobStateLoaded))
		if suToGlobal(u) {
			stdout("Triggered global unit %s stop", u.Name)
		} else {
			stopping = append(stopping, u.Name)
		}
	}

	exit = tryWaitForUnitStates(stopping, "stop", job.JobStateLoaded, getBlockAttempts(), os.Stdout)

	return
}
