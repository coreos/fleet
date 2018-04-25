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
	"strings"

	"github.com/spf13/cobra"

	"github.com/coreos/fleet/machine"
)

const (
	defaultListMachinesFields = "machine,ip,metadata"
)

var (
	listMachinesFieldsFlag string
	listMachinesFields     = map[string]machineToField{
		"machine": func(ms *machine.MachineState, full bool) string {
			return machineIDLegend(*ms, full)
		},
		"ip": func(ms *machine.MachineState, full bool) string {
			if len(ms.PublicIP) == 0 {
				return "-"
			}
			return ms.PublicIP
		},
		"metadata": func(ms *machine.MachineState, full bool) string {
			if len(ms.Metadata) == 0 {
				return "-"
			}
			return formatMetadata(ms.Metadata)
		},
	}
)

type machineToField func(ms *machine.MachineState, full bool) string

var cmdListMachines = &cobra.Command{
	Use:   "list-machines [-l|--full] [--no-legend]",
	Short: "Enumerate the current hosts in the cluster",
	Long: `Lists all active machines within the cluster. Previously active machines will not appear in this list.

For easily parsable output, you can remove the column headers:
fleetctl list-machines --no-legend

Output the list without truncation:
fleetctl list-machines --full`,
	Run: runWrapper(runListMachines),
}

func init() {
	cmdFleet.AddCommand(cmdListMachines)

	cmdListMachines.Flags().BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListMachines.Flags().BoolVar(&sharedFlags.Full, "l", false, "Shorthand for --full")
	cmdListMachines.Flags().BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListMachines.Flags().StringVar(&listMachinesFieldsFlag, "fields", defaultListMachinesFields, fmt.Sprintf("Columns to print for each Machine. Valid fields are %q", strings.Join(machineToFieldKeys(listMachinesFields), ",")))
}

func runListMachines(cCmd *cobra.Command, args []string) (exit int) {
	if listMachinesFieldsFlag == "" {
		stderr("Must define output format")
		return 1
	}

	cols := strings.Split(listMachinesFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listMachinesFields[s]; !ok {
			stderr("Invalid key in output format: %q", s)
			return 1
		}
	}

	machines, err := cAPI.Machines()
	if err != nil {
		stderr("Error retrieving list of active machines from fleet API (%v)", err)
		stderr("Possible issues:")
		stderr("  etcd is unhealthy and/or lost quorum")
		stderr("  connection cannot be established to any etcd servers")
		return 1
	}

	noLegend, _ := cCmd.Flags().GetBool("no-legend")
	if !noLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	full, _ := cCmd.Flags().GetBool("full")
	for _, ms := range machines {
		ms := ms
		var f []string
		for _, c := range cols {
			f = append(f, listMachinesFields[c](&ms, full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()

	return 0
}

func formatMetadata(metadata map[string]string) string {
	pairs := make([]string, len(metadata))
	idx := 0
	var sorted sort.StringSlice
	for k := range metadata {
		sorted = append(sorted, k)
	}
	sorted.Sort()
	for _, key := range sorted {
		value := metadata[key]
		if hasMetadataOperator(value) {
			pairs[idx] = fmt.Sprintf("%s%s", key, value)
		} else {
			pairs[idx] = fmt.Sprintf("%s=%s", key, value)
		}
		idx++
	}
	return strings.Join(pairs, ",")
}

func machineToFieldKeys(m map[string]machineToField) (keys []string) {
	for k, _ := range m {
		keys = append(keys, k)
	}
	return
}

func hasMetadataOperator(instr string) bool {
	for _, op := range []string{"<=", ">=", "!=", "==", "<", ">"} {
		if strings.HasPrefix(instr, op) {
			return true
		}
	}
	return false
}
