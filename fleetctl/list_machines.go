package main

import (
	"fmt"
	"strings"
)

var cmdListMachines = &Command{
	Name:    "list-machines",
	Summary: "Enumerate the current hosts in the cluster",
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
	cmdListMachines.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
}

func runListMachines(args []string) (exit int) {
	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, "MACHINE\tIP\tMETADATA")
	}

	for _, m := range registryCtl.GetActiveMachines() {
		mach := machineBootIDLegend(m, sharedFlags.Full)

		ip := m.PublicIP
		if len(ip) == 0 {
			ip = "-"
		}

		metadata := "-"
		if len(m.Metadata) != 0 {
			metadata = formatMetadata(m.Metadata)
		}

		fmt.Fprintf(out, "%s\t%s\t%s\n", mach, ip, metadata)
	}

	out.Flush()
	return 0
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
