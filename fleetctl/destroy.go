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
	"time"
)

var cmdDestroyUnit = &Command{
	Name:    "destroy",
	Summary: "Destroy one or more units in the cluster",
	Usage:   "UNIT...",
	Description: `Completely remove one or more running or submitted units from the cluster.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Destroyed units are impossible to start unless re-submitted.`,
	Run: runDestroyUnits,
}

func runDestroyUnits(args []string) (exit int) {
	units, err := findUnits(args)
	if err != nil {
		stderr("%v", err)
		return 1
	}

	for _, v := range units {
		err := cAPI.DestroyUnit(v.Name)
		if err != nil {
			stderr("Error destroying units: %v", err)
			exit = 1
			continue
		}

		if !sharedFlags.NoBlock {
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
				time.Sleep(500 * time.Millisecond)
			}
		}

		stdout("Destroyed %s", v.Name)
	}
	return
}
