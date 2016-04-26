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

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
)

func NewUnloadUnitCommand() cli.Command {
	return cli.Command{
		Name:      "unload",
		Usage:     "Unschedule one or more units in the cluster.",
		ArgsUsage: "UNIT...",
		Action:    makeActionWrapper(runUnloadUnit),
		Flags: []cli.Flag{
			cli.IntFlag{Name: "block-attempts", Value: 0, Usage: "Wait until the units are inactive, performing up to N attempts before giving up. A value of 0 indicates no limit."},
			cli.BoolFlag{Name: "no-block", Usage: "Do not wait until the units have become inactive before exiting."},
		},
	}
}

func runUnloadUnit(c *cli.Context, cAPI client.API) (exit int) {
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

	wait := make([]string, 0)
	for _, s := range units {
		if !suToGlobal(s) {
			if job.JobState(s.CurrentState) == job.JobStateInactive {
				log.Debugf("Target state of Unit(%s) already %s, skipping.", s.Name, job.JobStateInactive)
				continue
			}
		}

		log.Debugf("Setting target state of Unit(%s) to %s", s.Name, job.JobStateInactive)
		cAPI.SetUnitTargetState(s.Name, string(job.JobStateInactive))
		if suToGlobal(s) {
			stdout("Triggered global unit %s unload", s.Name)
		} else {
			wait = append(wait, s.Name)
		}
	}

	exit = tryWaitForUnitStates(wait, "unload", job.JobStateInactive, getBlockAttempts(c), os.Stdout, cAPI)

	return
}
