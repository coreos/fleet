package main

import (
	"fmt"
	"strings"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
)

func newListMachinesCommand() cli.Command {
	return cli.Command{
		Name:  "list-machines",
		Usage: "Enumerate the current hosts in the cluster",
		Description: `Lists all active machines within the cluster. Previously active machines will
not appear in this list.

For easily parsable output, you can remove the column headers:
fleetctl list-machines --no-legend

Output the list without truncation:
fleetctl list-machines --full`,
		Action: listMachinesAction,
		Flags: []cli.Flag{
			cli.BoolFlag{"full, l", "Do not ellipsize fields on output"},
			cli.BoolFlag{"no-legend", "Do not print a legend (column headers)"},
		},
	}
}

func listMachinesAction(c *cli.Context) {
	if !c.Bool("no-legend") {
		fmt.Fprintln(out, "MACHINE\tIP\tMETADATA")
	}

	full := c.Bool("full")

	for _, m := range registryCtl.GetActiveMachines() {
		mach := m.BootId
		if !full {
			mach = ellipsize(mach, 8)
		}

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
