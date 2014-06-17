package main

import (
	"fmt"
	"os"
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
		fmt.Fprintf(os.Stderr, "Must define output format\n")
		return 1
	}

	cols := strings.Split(listMachinesFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listMachinesFields[s]; !ok {
			fmt.Fprintf(os.Stderr, "Invalid key in output format: %q\n", s)
			return 1
		}
	}

	machines, sortable, err := findAllMachines()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving list of active machines: %v\n", err)
		return 1
	}

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, name := range sortable {
		var f []string
		ms := machines[name]
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
	for key, value := range metadata {
		pairs[idx] = fmt.Sprintf("%s=%s", key, value)
		idx++
	}
	return strings.Join(pairs, ",")
}

// findAllMachines returns a map describing all the machines in the Registry, and a
// sort.StringSlice indicating their sorted order (based on their respective
// machine IDs). It returns any error encountered in communicating with the Registry.
func findAllMachines() (machines map[string]machine.MachineState, sortable sort.StringSlice, err error) {
	machines = make(map[string]machine.MachineState, 0)
	mm, err := fc.GetActiveMachines()
	if err != nil {
		return
	}

	for _, m := range mm {
		machines[m.ID] = m
		sortable = append(sortable, m.ID)
	}

	sortable.Sort()

	return
}

func machineToFieldKeys(m map[string]machineToField) (keys []string) {
	for k, _ := range m {
		keys = append(keys, k)
	}
	return
}
