package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/coreos/fleet/machine"
)

const (
	defaultListMachinesFields = "machine,ip,metadata"
)

var (
	listMachinesFieldsFlag string
	cmdListMachines        = &Command{
		Name:    "list-machines",
		Summary: "Enumerate the current hosts in the cluster",
		Usage:   "[-l|--full] [--no-legend]",
		Description: `Lists all active machines within the cluster. Previously active machines will not appear in this list.

For easily parsable output, you can remove the column headers:
	fleetctl list-machines --no-legend

Output the list without truncation:
	fleetctl list-machines --full`,
		Run: runListMachines,
	}

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

type machineToField func(ms *machine.MachineState, full bool) string

func init() {
	cmdListMachines.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListMachines.Flags.BoolVar(&sharedFlags.Full, "l", false, "Shorthand for --full")
	cmdListMachines.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListMachines.Flags.StringVar(&listMachinesFieldsFlag, "fields", defaultListMachinesFields, fmt.Sprintf("Columns to print for each Machine. Valid fields are %q", strings.Join(machineToFieldKeys(listMachinesFields), ",")))
}

func runListMachines(args []string) (exit int) {
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

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, ms := range machines {
		ms := ms
		var f []string
		for _, c := range cols {
			f = append(f, listMachinesFields[c](&ms, sharedFlags.Full))
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
