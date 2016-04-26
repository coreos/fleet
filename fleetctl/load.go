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

func NewLoadUnitsCommand() cli.Command {
	return cli.Command{
		Name:      "load",
		Usage:     "Schedule one or more units in the cluster, first submitting them if necessary.",
		ArgsUsage: "[--no-block|--block-attempts=N] UNIT...",
		Description: `Load one or many units in the cluster into systemd, but do not start.

Select units to load by glob matching for units in the current working directory 
or matching the names of previously submitted units.

For units which are not global, load operations are performed synchronously,
which means fleetctl will block until it detects that the unit(s) have
transitioned to a loaded state. This behaviour can be configured with the
respective --block-attempts and --no-block options. Load operations on global
units are always non-blocking.`,
		Action: makeActionWrapper(runLoadUnits),
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "sign", Usage: "DEPRECATED - this option cannot be used"},
			cli.IntFlag{Name: "block-attempts", Value: 0, Usage: "ait until the jobs are loaded, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units."},
			cli.BoolFlag{Name: "no-block", Usage: "Do not wait until the jobs have been loaded before exiting. Always the case for global units."},
			cli.BoolFlag{Name: "replace", Usage: "Replace the old scheduled units in the cluster with new versions."},
		},
	}
}

func runLoadUnits(c *cli.Context, cAPI client.API) (exit int) {
	args := c.Args()
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}

	if err := lazyCreateUnits(c, cAPI); err != nil {
		stderr("Error creating units: %v", err)
		return 1
	}

	triggered, err := lazyLoadUnits(args, cAPI)
	if err != nil {
		stderr("Error loading units: %v", err)
		return 1
	}

	var loading []string
	for _, u := range triggered {
		if suToGlobal(*u) {
			stdout("Triggered global unit %s load", u.Name)
		} else {
			loading = append(loading, u.Name)
		}
	}

	exit = tryWaitForUnitStates(loading, "load", job.JobStateLoaded, getBlockAttempts(c), os.Stdout, cAPI)

	return
}
