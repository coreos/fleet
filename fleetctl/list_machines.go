package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coreos/fleet/machine"
)

var cmdListMachines = &Command{
	Name:    "list-machines",
	Summary: "Enumerate the current hosts in the cluster",
	Usage:   "[-l|--full] [--no-legend]",
	Description: `Lists all active machines within the cluster. Previously active machines will
not appear in this list.

For easily parsable output, you can remove the column headers:
	fleetctl list-machines --no-legend

Output the list without truncation:
	fleetctl list-machines --full`,
	Run: runListMachines,
}

func init() {
	cmdListMachines.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListMachines.Flags.BoolVar(&sharedFlags.Full, "l", false, "Shorthand for --full")
	cmdListMachines.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
}

func runListMachines(args []string) (exit int) {
	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, "MACHINE\tIP\tMETADATA")
	}

	machines, sortable, err := findAllMachines()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving list of active machines: %v\n", err)
		return 1
	}
	for _, m := range sortable {
		mach := machines[m]

		ml := machineIDLegend(mach, sharedFlags.Full)

		ip := mach.PublicIP
		if len(ip) == 0 {
			ip = "-"
		}

		metadata := "-"
		if len(mach.Metadata) != 0 {
			metadata = formatMetadata(mach.Metadata)
		}

		fmt.Fprintf(out, "%s\t%s\t%s\n", ml, ip, metadata)
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
	return strings.Join(pairs, ", ")
}

// findAllMachines returns a map describing all the machines in the Registry, and a
// sort.StringSlice indicating their sorted order (based on their respective
// machine IDs). It returns any error encountered in communicating with the Registry.
func findAllMachines() (machines map[string]machine.MachineState, sortable sort.StringSlice, err error) {
	machines = make(map[string]machine.MachineState, 0)
	mm, err := registryCtl.GetActiveMachines()
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
