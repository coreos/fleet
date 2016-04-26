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
)

func NewStartCommand() cli.Command {
	return cli.Command{
		Name:      "start",
		Usage:     "Instruct systemd to start one or more units in the cluster, first submitting and loading if necessary.",
		ArgsUsage: "[--no-block|--block-attempts=N] UNIT...",
		Description: `Start one or many units on the cluster. Select units to start by glob matching for units in the current working directory or matching names of previously submitted units.

For units which are not global, start operations are performed synchronously,
which means fleetctl will block until it detects that the unit(s) have
transitioned to a started state. This behaviour can be configured with the
respective --block-attempts and --no-block options. Start operations on global
units are always non-blocking.

Start a single unit:
       fleetctl start foo.service

Start an entire directory of units with glob matching:
       fleetctl start myservice/*

You may filter suitable hosts based on metadata provided by the machine.
Machine metadata is located in the fleet configuration file.`,
		Action: makeActionWrapper(runStartUnit),
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "sign", Usage: "DEPRECATED - this option cannot be used"},
			cli.IntFlag{Name: "block-attempts", Value: 0, Usage: "Wait until the units are launched, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units."},
			cli.BoolFlag{Name: "no-block", Usage: "Do not wait until the units have launched before exiting. Always the case for global units."},
			cli.BoolFlag{Name: "replace", Usage: "Replace the already started units in the cluster with new versions."},
		},
	}
}

func runStartUnit(c *cli.Context, cAPI client.API) (exit int) {
	args := c.Args()
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}

	if err := lazyCreateUnits(c, cAPI); err != nil {
		stderr("Error creating units: %v", err)
		return 1
	}

	triggered, err := lazyStartUnits(args, cAPI)
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

	exit = tryWaitForUnitStates(starting, "start", job.JobStateLaunched, getBlockAttempts(c), os.Stdout, cAPI)
	return
}
