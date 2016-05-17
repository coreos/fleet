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
	"strconv"

	"github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
)

func NewJournalCommand() cli.Command {
	return cli.Command{
		Name:      "journal",
		Usage:     "Print the journal of a unit in the cluster to stdout",
		ArgsUsage: "[--lines=N] [--ssh-port=N] [-f|--follow] [--output=STRING] <unit>",
		Action:    makeActionWrapper(runJournal),
		Description: `Outputs the journal of a unit by connecting to the machine that the unit occupies. 

Read the last 10 lines:
       fleetctl journal foo.service

Read the last 100 lines:
       fleetctl journal --lines 100 foo.service

This command does not work with global units.`,
		Flags: []cli.Flag{
			cli.IntFlag{Name: "lines", Value: 10, Usage: "Number of recent log lines to return"},
			cli.BoolFlag{Name: "follow, f", Usage: "Continuously print new entries as they are appended to the journal."},
			cli.IntFlag{Name: "ssh-port", Value: 22, Usage: "Connect to remote hosts over SSH using this TCP port"},
			cli.BoolFlag{Name: "sudo", Usage: "Execute journal command with sudo"},
			cli.StringFlag{Name: "output", Value: "short", Usage: "Output mode. This will be passed unaltered to journalctl on the remote host, and hence supports the same modes as that command."},
		},
	}
}

func runJournal(c *cli.Context, cAPI client.API) (exit int) {
	args := c.Args()
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

	cmd := []string{"journalctl", "--unit", name, "--no-pager", "-n", strconv.Itoa(c.Int("lines")), "--output", c.String("output")}

	if c.Bool("sudo") {
		cmd = append([]string{"sudo"}, cmd...)
	}

	if c.Bool("follow") {
		cmd = append(cmd, "-f")
	}

	return runCommand(c, u.MachineID, cmd[0], cmd[1:]...)
}
