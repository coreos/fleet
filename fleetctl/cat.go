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

	"github.com/coreos/fleet/schema"
)

var (
	cmdCatUnit = &Command{
		Name:    "cat",
		Summary: "Output the contents of a submitted unit",
		Usage:   "UNIT",
		Description: `Outputs the unit file that is currently loaded in the cluster. Useful to verify
the correct version of a unit is running.`,
		Run: runCatUnit,
	}
)

func runCatUnit(args []string) (exit int) {
	if len(args) != 1 {
		stderr("One unit file must be provided")
		return 1
	}

	name := unitNameMangle(args[0])
	u, err := cAPI.Unit(name)
	if err != nil {
		stderr("Error retrieving Unit %s: %v", name, err)
		return 1
	}
	if u == nil {
		stderr("Unit %s not found", name)
		return 1
	}

	uf := schema.MapSchemaUnitOptionsToUnitFile(u.Options)

	// Must not add a newline here. The contents of the unit file
	// must not be modified.
	fmt.Print(uf.String())

	return
}
