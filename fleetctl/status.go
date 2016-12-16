// Copyright 2014 The fleet Authors
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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/schema"
)

var cmdStatus = &cobra.Command{
	Use:   "status [--ssh-port=N] UNIT...",
	Short: "Output the status of one or more units in the cluster",
	Long: `Output the status of one or more units currently running in the cluster.
Supports glob matching of units in the current working directory or matches
previously started units.

Show status of a single unit:
	fleetctl status foo.service

Show status of an entire directory with glob matching:
fleetctl status myservice/*

This command does not work with global units.`,
	Run: runWrapper(runStatusUnit),
}

func init() {
	cmdFleet.AddCommand(cmdStatus)

	cmdStatus.Flags().IntVar(&sharedFlags.SSHPort, "ssh-port", 22, "Connect to remote hosts over SSH using this TCP port.")
}

func runStatusUnit(cCmd *cobra.Command, args []string) (exit int) {
	globalUnits := make([]schema.Unit, 0)
	for i, arg := range args {
		name := unitNameMangle(arg)
		unit, err := cAPI.Unit(name)
		if err != nil {
			stderr("Error retrieving unit: %v", err)
			return 1
		}

		if unit == nil {
			stderr("Unit %s does not exist.", name)
			return 1
		} else if suToGlobal(*unit) {
			globalUnits = append(globalUnits, *unit)
			continue
		} else if job.JobState(unit.CurrentState) == job.JobStateInactive {
			stderr("Unit %s does not appear to be loaded.", unit.Name)
			return 1
		}

		// This extra newline is here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		if exitVal := runCommand(cCmd, unit.MachineID, "systemctl", "status", "-l", unit.Name); exitVal != 0 {
			exit = exitVal
			break
		}
	}

	if err := cmdGlobalMachineState(cCmd, globalUnits); err != nil {
		stderr("Error retrieving machine state for global units: %v", err)
		return 1
	}

	return
}
