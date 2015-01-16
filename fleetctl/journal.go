/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"fmt"

	"github.com/coreos/fleet/job"
)

var (
	flagLines  int
	flagFollow bool
	flagSudo   bool
	cmdJournal = &Command{
		Name:    "journal",
		Summary: "Print the journal of a unit in the cluster to stdout",
		Usage:   "[--lines=N] [-f|--follow] <unit>",
		Run:     runJournal,
		Description: `Outputs the journal of a unit by connecting to the machine that the unit occupies.

Read the last 10 lines:
	fleetctl journal foo.service

Read the last 100 lines:
	fleetctl journal --lines 100 foo.service

This command does not work with global units.`,
	}
)

func init() {
	cmdJournal.Flags.IntVar(&flagLines, "lines", 10, "Number of recent log lines to return")
	cmdJournal.Flags.BoolVar(&flagFollow, "follow", false, "Continuously print new entries as they are appended to the journal.")
	cmdJournal.Flags.BoolVar(&flagFollow, "f", false, "Shorthand for --follow")
	cmdJournal.Flags.BoolVar(&flagSudo, "sudo", false, "Execute journal command with sudo")
}

func runJournal(args []string) (exit int) {
	if len(args) != 1 {
		stderr("One unit file must be provided.")
		return 1
	}
	name := unitNameMangle(args[0])

	u, err := cAPI.Unit(name)
	if err != nil {
		stderr("Error retrieving unit %s: %v", name, err)
		return 1
	} else if u == nil {
		stderr("Unit %s does not exist.", name)
		return 1
	} else if suToGlobal(*u) {
		stderr("Unable to retrieve journal of global unit %s.", name)
		return 1
	} else if job.JobState(u.CurrentState) == job.JobStateInactive {
		stderr("Unit %s does not appear to be running.", name)
		return 1
	}

	command := fmt.Sprintf("journalctl --unit %s --no-pager -n %d", name, flagLines)

	if flagSudo {
		command = "sudo " + command
	}

	if flagFollow {
		command += " -f"
	}

	return runCommand(command, u.MachineID)
}
