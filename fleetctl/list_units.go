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

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

const (
	defaultListUnitsFields = "unit,machine,active,sub"
)

var (
	listUnitsFieldsFlag string
	cmdListUnits        = &Command{
		Name:    "list-units",
		Summary: "List the current state of units in the cluster",
		Usage:   "[--no-legend] [-l|--full] [--fields]",
		Description: `Lists the state of all units in the cluster loaded onto a machine.

For easily parsable output, you can remove the column headers:
	fleetctl list-units --no-legend

Output the list without ellipses:
	fleetctl list-units --full

Or, choose the columns to display:
	fleetctl list-units --fields=unit,machine`,
		Run: runListUnits,
	}

	listUnitsFields = map[string]usToField{
		"unit": func(us *schema.UnitState, full bool) string {
			if us == nil {
				return "-"
			}
			return us.Name
		},
		"load": func(us *schema.UnitState, full bool) string {
			if us == nil {
				return "-"
			}
			return us.SystemdLoadState
		},
		"active": func(us *schema.UnitState, full bool) string {
			if us == nil {
				return "-"
			}
			return us.SystemdActiveState
		},
		"sub": func(us *schema.UnitState, full bool) string {
			if us == nil {
				return "-"
			}
			return us.SystemdSubState
		},
		"machine": func(us *schema.UnitState, full bool) string {
			if us == nil || us.MachineID == "" {
				return "-"
			}
			ms := cachedMachineState(us.MachineID)
			if ms == nil {
				ms = &machine.MachineState{ID: us.MachineID}
			}
			return machineFullLegend(*ms, full)
		},
		"hash": func(us *schema.UnitState, full bool) string {
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

type usToField func(us *schema.UnitState, full bool) string

func init() {
	cmdListUnits.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnits.Flags.BoolVar(&sharedFlags.Full, "l", false, "Shorthand for --full")
	cmdListUnits.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnits.Flags.StringVar(&listUnitsFieldsFlag, "fields", defaultListUnitsFields, fmt.Sprintf("Columns to print for each Unit. Valid fields are %q", strings.Join(usToFieldKeys(listUnitsFields), ",")))
}

func runListUnits(args []string) (exit int) {
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

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, us := range states {
		var f []string
		for _, c := range cols {
			f = append(f, listUnitsFields[c](us, sharedFlags.Full))
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
