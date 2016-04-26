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

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
)

func NewDestroyCommand() cli.Command {
	return cli.Command{
		Name:      "destroy",
		Usage:     "Destroy one or more units in the cluster",
		ArgsUsage: "UNIT...",
		Description: `Completely remove one or more running or submitted units from the cluster.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Destroyed units are impossible to start unless re-submitted.`,
		Action: makeActionWrapper(runDestroyUnits),
	}
}

func runDestroyUnits(c *cli.Context, cAPI client.API) (exit int) {
	args := c.Args()
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}

	units, err := findUnits(args, cAPI)
	if err != nil {
		stderr("%v", err)
		return 1
	}

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

		if c.Bool("no-block") {
			attempts := c.Int("block-attempts")
			retry := func() bool {
				if c.Int("block-attempts") < 1 {
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

		stdout("Destroyed %s", v.Name)
	}

	return
}
