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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

const (
	defaultListUnitsFields = "unit,machine,active,sub,uptime"
	tmFormatString         = "2006-01-02 03:04:05 PM MST"
)

var (
	listUnitsFieldsFlag string
	listUnitsFields     = map[string]usToField{
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
		"uptime": func(us *schema.UnitState, full bool) string {
			if us == nil || us.SystemdActiveState != "active" {
				return "-"
			}
			// SystemdActiveEnterTimestamp is in microseconds, while time.Unix
			// requires the 2nd parameter as value in nanoseconds.
			ts, _ := strconv.Atoi(us.SystemdActiveEnterTimestamp)
			tm := time.Unix(0, int64(ts)*1000)
			duration := time.Now().Sub(tm)
			return fmt.Sprintf("%s, Since %ss", tm.Format(tmFormatString), strings.Split(duration.String(), ".")[0])
		},
	}
)

type usToField func(us *schema.UnitState, full bool) string

var cmdListUnits = &cobra.Command{
	Use:   "list-units [--no-legend] [-l|--full] [--fields]",
	Short: "List the current state of units in the cluster",
	Long: `Lists the state of all units in the cluster loaded onto a machine.

For easily parsable output, you can remove the column headers:
fleetctl list-units --no-legend

Output the list without ellipses:
fleetctl list-units --full

Or, choose the columns to display:
fleetctl list-units --fields=unit,machine`,
	Run: runWrapper(runListUnits),
}

func init() {
	cmdFleet.AddCommand(cmdListUnits)

	cmdListUnits.Flags().BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnits.Flags().BoolVar(&sharedFlags.Full, "l", false, "Shorthand for --full")
	cmdListUnits.Flags().BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnits.Flags().StringVar(&listUnitsFieldsFlag, "fields", defaultListUnitsFields, fmt.Sprintf("Columns to print for each Unit. Valid fields are %q", strings.Join(usToFieldKeys(listUnitsFields), ",")))
}

func runListUnits(cCmd *cobra.Command, args []string) (exit int) {
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

	noLegend, _ := cCmd.Flags().GetBool("no-legend")
	if !noLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	full, _ := cCmd.Flags().GetBool("full")
	for _, us := range states {
		var f []string
		for _, c := range cols {
			f = append(f, listUnitsFields[c](us, full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return 0
}

func usToFieldKeys(m map[string]usToField) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return
}
