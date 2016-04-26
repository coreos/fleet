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
	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
)

func NewSubmitUnitCommand() cli.Command {
	return cli.Command{
		Name:      "submit",
		Usage:     "Upload one or more units to the cluster without starting them",
		ArgsUsage: "UNIT...",
		Description: `Upload one or more units to the cluster without starting them. Useful for validating units before they are started.

This operation is idempotent; if a named unit already exists in the cluster, it will not be resubmitted.

Submit a single unit:
       fleetctl submit foo.service

Submit a directory of units with glob matching:
       fleetctl submit myservice/*`,
		Action: makeActionWrapper(runSubmitUnits),
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "sign", Usage: "DEPRECATED - this option cannot be used"},
			cli.BoolFlag{Name: "replace", Usage: "Replace the old submitted units in the cluster with new versions."},
		},
	}
}

func runSubmitUnits(c *cli.Context, cAPI client.API) (exit int) {
	args := c.Args()
	if len(args) == 0 {
		stderr("No units given")
		return 0
	}

	if err := lazyCreateUnits(c, cAPI); err != nil {
		stderr("Error creating units: %v", err)
		exit = 1
	}

	return
}
