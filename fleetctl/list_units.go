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
	"sort"
	"strings"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

const (
	defaultListUnitsFields = "unit,machine,active,sub"
)

func NewListUnitsCommand() cli.Command {
	return cli.Command{
		Name:      "list-units",
		Usage:     "List the current state of units in the cluster",
		ArgsUsage: "[--no-legend] [-l|--full] [--fields]",
		Description: `Lists the state of all units in the cluster loaded onto a machine.

For easily parsable output, you can remove the column headers:
       fleetctl list-units --no-legend

Output the list without ellipses:
       fleetctl list-units --full

Or, choose the columns to display:
       fleetctl list-units --fields=unit,machine`,
		Action: makeActionWrapper(runListUnits),
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "full, l", Usage: "Do not ellipsize fields on output"},
			cli.BoolFlag{Name: "no-legend", Usage: "Do not print a legend (column headers)"},
			cli.StringFlag{Name: "fields", Value: defaultListUnitsFields, Usage: fmt.Sprintf("Columns to print for each Unit. Valid fields are %q", strings.Join(usToFieldKeys(listUnitsFields), ","))},
		},
	}
}

var (
	listUnitsFields = map[string]usToField{
		"unit": func(us *schema.UnitState, full bool, cAPI client.API) string {
			if us == nil {
				return "-"
			}
			return us.Name
		},
		"load": func(us *schema.UnitState, full bool, cAPI client.API) string {
			if us == nil {
				return "-"
			}
			return us.SystemdLoadState
		},
		"active": func(us *schema.UnitState, full bool, cAPI client.API) string {
			if us == nil {
				return "-"
			}
			return us.SystemdActiveState
		},
		"sub": func(us *schema.UnitState, full bool, cAPI client.API) string {
			if us == nil {
				return "-"
			}
			return us.SystemdSubState
		},
		"machine": func(us *schema.UnitState, full bool, cAPI client.API) string {
			if us == nil || us.MachineID == "" {
				return "-"
			}
			ms := cachedMachineState(us.MachineID, cAPI)
			if ms == nil {
				ms = &machine.MachineState{ID: us.MachineID}
			}
			return machineFullLegend(*ms, full)
		},
		"hash": func(us *schema.UnitState, full bool, cAPI client.API) string {
			if us == nil || us.Hash == "" {
				return "-"
			}
			if !full {
				return us.Hash[:7]
			}
			return us.Hash
		},
	}
)

type usToField func(us *schema.UnitState, full bool, cAPI client.API) string

func runListUnits(c *cli.Context, cAPI client.API) (exit int) {
	listUnitsFieldsFlag := c.String("fields")
	if listUnitsFieldsFlag == "" {
		stderr("Must define output format")
		return 1
	}

	cols := strings.Split(listUnitsFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listUnitsFields[s]; !ok {
			stderr("Invalid key in output format: %q", s)
			return 1
		}
	}

	states, err := cAPI.UnitStates()
	if err != nil {
		stderr("Error retrieving list of units from repository: %v", err)
		return 1
	}

	if !c.Bool("no-legend") {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, us := range states {
		var f []string
		for _, col := range cols {
			f = append(f, listUnitsFields[col](us, c.Bool("full"), cAPI))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()

	return
}

func usToFieldKeys(m map[string]usToField) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return
}
