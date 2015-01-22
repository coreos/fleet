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
)

var (
	cmdLoadUnits = &Command{
		Name:    "load",
		Summary: "Schedule one or more units in the cluster, first submitting them if necessary.",
		Usage:   "[--no-block|--block-attempts=N] UNIT...",
		Description: `Load one or many units in the cluster into systemd, but do not start.

Select units to load by glob matching for units in the current working directory 
or matching the names of previously submitted units.

For units which are not global, load operations are performed synchronously,
which means fleetctl will block until it detects that the unit(s) have
transitioned to a loaded state. This behaviour can be configured with the
respective --block-attempts and --no-block options. Load operations on global
units are always non-blocking.`,
		Run: runLoadUnits,
	}
)

func init() {
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "DEPRECATED - this option cannot be used")
	cmdLoadUnits.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the jobs are loaded, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units.")
	cmdLoadUnits.Flags.BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have been loaded before exiting. Always the case for global units.")
}

func runLoadUnits(args []string) (exit int) {
	if err := lazyCreateUnits(args); err != nil {
		stderr("Error creating units: %v", err)
		return 1
	}

	triggered, err := lazyLoadUnits(args)
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

	if !sharedFlags.NoBlock {
		errchan := waitForUnitStates(loading, job.JobStateLoaded, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			stderr("Error waiting for units: %v", err)
			exit = 1
		}
	} else {
		for _, name := range loading {
			stdout("Triggered unit %s load", name)
		}
	}

	return
}
