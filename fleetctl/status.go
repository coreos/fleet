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
	"fmt"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/schema"
)

var cmdStatusUnits = &Command{
	Name:    "status",
	Summary: "Output the status of one or more units in the cluster",
	Usage:   "[--ssh-port=N] UNIT...",
	Description: `Output the status of one or more units currently running in the cluster.
Supports glob matching of units in the current working directory or matches
previously started units.

Show status of a single unit:
	fleetctl status foo.service

Show status of an entire directory with glob matching:
fleetctl status myservice/*

This command does not work with global units.`,
	Run: runStatusUnits,
}

func init() {
	cmdStatusUnits.Flags.IntVar(&sharedFlags.SSHPort, "ssh-port", 22, "Connect to remote hosts over SSH using this TCP port.")
}

func runStatusUnits(args []string) (exit int) {
	if len(args) == 0 {
		stderr("One unit file must be provided.")
		return 1
	}
	units, err := cAPI.Units()
	if err != nil {
		stderr("Error retrieving unit: %v", err)
		return 1
	}

	uMap := make(map[string]*schema.Unit, len(args))
	for _, u := range units {
		if u != nil {
			u := u
			uMap[u.Name] = u
		}
	}

	names := make([]string, len(args))
	for i, arg := range args {
		name := unitNameMangle(arg)
		names[i] = name

		u, ok := uMap[name]
		if !ok {
			stderr("Unit %s does not exist.", name)
			return 1
		} else if suToGlobal(*u) {
			stderr("Unable to determine status of global unit %s.", name)
			return 1
		} else if job.JobState(u.CurrentState) == job.JobStateInactive {
			stderr("Unit %s does not appear to be loaded.", name)
			return 1
		}
	}

	for i, name := range names {
		// This extra newline is here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		if exit = runCommand(uMap[name].MachineID, "systemctl", "status", "-l", name); exit != 0 {
			break
		}
	}
	return
}
