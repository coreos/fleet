// Copyright 2016 The fleet Authors
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

	"github.com/spf13/cobra"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/schema"
)

var cmdRestart = &cobra.Command{
	Use:   "restart [--block-attempts=N] UNIT...",
	Short: "Instruct systemd to rolling restart one or more units in the cluster.",
	Long: `Restarts one or more units in the cluster. If they are stopped it starts them.

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
	Run: runWrapper(runRestartUnit),
}

func init() {
	cmdFleet.AddCommand(cmdRestart)

	cmdRestart.Flags().IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are stopped/started, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units.")
}

func runRestartUnit(cCmd *cobra.Command, args []string) (exit int) {
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}
	units, err := findUnits(args)
	if err != nil {
		stderr("%v", err)
		return 1
	}

	if err := lazyCreateUnits(cCmd, args); err != nil {
		stderr("Error creating units: %v", err)
		return 1
	}

	globalUnits := make([]schema.Unit, 0)
	for _, unit := range units {
		if suToGlobal(unit) {
			globalUnits = append(globalUnits, unit)
			continue
		}
		if job.JobState(unit.CurrentState) == job.JobStateInactive {
			stderr("Unable to restart unit %s in state %s", unit.Name, job.JobStateInactive)
			continue
		} else if job.JobState(unit.CurrentState) == job.JobStateLoaded {
			log.Infof("Unit(%s) already %s, starting.", unit.Name, job.JobStateLoaded)

			exit = setUnitStateAndWait(unit, job.JobStateLaunched, getBlockAttempts(cCmd))
			if exit == 1 {
				return exit
			}
			continue
		} else {
			//stop and start it
			exit = setUnitStateAndWait(unit, job.JobStateLoaded, getBlockAttempts(cCmd))
			if exit == 1 {
				return exit
			}
			exit = setUnitStateAndWait(unit, job.JobStateLaunched, getBlockAttempts(cCmd))
			if exit == 1 {
				return exit
			}
		}
		log.Infof("Unit(%s) was restarted.", unit.Name)
	}

	if err := cmdGlobalMachineState(cCmd, globalUnits); err != nil {
		stderr("Error restarting global units %v err:%v", globalUnits, err)
		return 1
	}

	return
}

func setUnitStateAndWait(unit schema.Unit, targetState job.JobState, blockAttempts int) (exit int) {
	err := cAPI.SetUnitTargetState(unit.Name, string(targetState))
	if err != nil {
		stderr("Error setting target state for unit %s: %v", unit.Name, err)
		return 1
	}

	err = tryWaitForUnitStates([]string{unit.Name}, "restart", job.JobStateLaunched, blockAttempts, os.Stdout)
	if err != nil {
		stderr("Error waiting for unit states, exit status: %v", err)
		return 1
	}

	return 0
}
