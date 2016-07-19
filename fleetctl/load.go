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
	"os"

	"github.com/spf13/cobra"

	"github.com/coreos/fleet/job"
)

var cmdLoad = &cobra.Command{
	Use:   "load [--no-block|--block-attempts=N] UNIT...",
	Short: "Schedule one or more units in the cluster, first submitting them if necessary.",
	Long: `Load one or many units in the cluster into systemd, but do not start.

Select units to load by glob matching for units in the current working directory 
or matching the names of previously submitted units.

For units which are not global, load operations are performed synchronously,
which means fleetctl will block until it detects that the unit(s) have
transitioned to a loaded state. This behaviour can be configured with the
respective --block-attempts and --no-block options. Load operations on global
units are always non-blocking.`,
	Run: runWrapper(runLoadUnit),
}

func init() {
	cmdFleet.AddCommand(cmdLoad)

	cmdLoad.Flags().BoolVar(&sharedFlags.Sign, "sign", false, "DEPRECATED - this option cannot be used")
	cmdLoad.Flags().IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Wait until the jobs are loaded, performing up to N attempts before giving up. A value of 0 indicates no limit. Does not apply to global units.")
	cmdLoad.Flags().BoolVar(&sharedFlags.NoBlock, "no-block", false, "Do not wait until the jobs have been loaded before exiting. Always the case for global units.")
	cmdLoad.Flags().BoolVar(&sharedFlags.Replace, "replace", false, "Replace the old scheduled units in the cluster with new versions.")
}

func runLoadUnit(cCmd *cobra.Command, args []string) (exit int) {
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}

	if err := lazyCreateUnits(cCmd, args); err != nil {
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

	exitVal := tryWaitForUnitStates(loading, "load", job.JobStateLoaded, getBlockAttempts(cCmd), os.Stdout)
	if exitVal != 0 {
		stderr("Error waiting for unit states, exit status: %d", exitVal)
		return 1
	}

	return 0
}
