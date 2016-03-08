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
	wait := make([]string, 0)

	for _, arg := range args {
		name := unitNameMangle(arg)
		unit, err := cAPI.Unit(name)
		if err != nil {
			stderr("Error retrieving unit: %v", err)
			return 1
		}

		if unit == nil {
			stderr("Unit %s does not exist.", name)
			continue
		}

		if !suToGlobal(*unit) {
			if job.JobState(unit.CurrentState) == job.JobStateInactive {
				log.Debugf("Target state of Unit(%s) already %s, skipping.", unit.Name, job.JobStateInactive)
				continue
			}
		}

		log.Debugf("Setting target state of Unit(%s) to %s", unit.Name, job.JobStateInactive)
		cAPI.SetUnitTargetState(unit.Name, string(job.JobStateInactive))
		if suToGlobal(*unit) {
			stdout("Triggered global unit %s unload", unit.Name)
		} else {
			wait = append(wait, unit.Name)
		}
	}

	if !sharedFlags.NoBlock {
		errchan := waitForUnitStates(wait, job.JobStateInactive, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			stderr("Error waiting for units: %v", err)
			exit = 1
		}
	} else {
		for _, name := range wait {
			stdout("Triggered unit %s unload", name)
		}
	}

	return
}
