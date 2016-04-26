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

	"github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/machine"
)

func NewListMachinesCommand() cli.Command {
	return cli.Command{
		Name:  "list-machines",
		Usage: "Enumerate the current hosts in the cluster",
		Description: `Lists all active machines within the cluster. Previously active machines will not appear in this list.

For easily parsable output, you can remove the column headers:
       fleetctl list-machines --no-legend

Output the list without truncation:
       fleetctl list-machines --full`,
		ArgsUsage: "[-l|--full] [--no-legend]",
		Action:    makeActionWrapper(runListMachines),
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "full, l", Usage: "Output the list without truncation"},
			cli.BoolFlag{Name: "no-legend", Usage: "Remove the column headers"},
			cli.StringFlag{Name: "fields", Value: defaultListMachinesFields, Usage: fmt.Sprintf("Columns to print for each Machine. Valid fields are %s", defaultListMachinesFields)},
		},
	}
}

var (
	//listMachinesFieldsFlag string
	// Update defaultListMachinesFields if you add a new field here
	listMachinesFields = map[string]machineToField{
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

const (
	defaultListMachinesFields = "machine,ip,metadata"
)

type machineToField func(ms *machine.MachineState, full bool) string

func runListMachines(c *cli.Context, cAPI client.API) (exit int) {
	listMachinesFieldsFlag := c.String("fields")
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
		stderr("Error retrieving list of active machines: %v", err)
		return 1
	}

	if !c.Bool("no-legend") {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, ms := range machines {
		ms := ms
		var f []string
		for _, col := range cols {
			f = append(f, listMachinesFields[col](&ms, c.Bool("full")))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()

	return
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
		pairs[idx] = fmt.Sprintf("%s=%s", key, value)
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
