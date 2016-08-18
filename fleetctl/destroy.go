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
	"time"

	"github.com/spf13/cobra"

	"github.com/coreos/fleet/client"
)

var cmdDestroy = &cobra.Command{
	Use:   "destroy UNIT...",
	Short: "Destroy one or more units in the cluster",
	Long: `Completely remove one or more running or submitted units from the cluster.

Instructs systemd on the host machine to destroy the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Destroyed units are impossible to start unless re-submitted.`,
	Run: runWrapper(runDestroyUnit),
}

func init() {
	cmdFleet.AddCommand(cmdDestroy)

	cmdDestroy.Flags().IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the units are destroyed, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units.")
	cmdDestroy.Flags().BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the units are destroyed before exiting. Always the case for global units.")
	cmdDestroy.Flags().IntVar(&sharedFlags.MaxPrintUnits, "max-print-units", 0, "Set maximum number of units to be printed")
}

func runDestroyUnit(cCmd *cobra.Command, args []string) (exit int) {
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}

	units, err := findUnits(args)
	if err != nil {
		stderr("%v", err)
		return 1
	}

	if len(units) == 0 {
		stderr("Units not found in registry")
		return 0
	}

	allUnits := make([]string, 0)
	for _, v := range units {
		err := cAPI.DestroyUnit(v.Name)
		if err != nil {
			// Ignore 'Unit does not exist' error
			if client.IsErrorUnitNotFound(err) {
				continue
			}
			stderr("Error destroying units: %v", err)
			exit = 1
			continue
		}

		if sharedFlags.NoBlock {
			attempts := sharedFlags.BlockAttempts
			retry := func() bool {
				if sharedFlags.BlockAttempts < 1 {
					return true
				}
				attempts--
				if attempts == 0 {
					return false
				}
				return true
			}

			for retry() {
				u, err := cAPI.Unit(v.Name)
				if err != nil {
					stderr("Error destroying units: %v", err)
					exit = 1
					break
				}

				if u == nil {
					break
				}
				time.Sleep(defaultSleepTime)
			}
		}
		allUnits = append(allUnits, v.Name)
	}

	oUnits := omitUnits(allUnits, sharedFlags.MaxPrintUnits)
	for _, u := range oUnits {
		stdout("Destroyed %s", u)
	}

	return
}
