package main

import (
	"fmt"
	"strings"

	"github.com/codegangsta/cli"
)

func newListMachinesCommand() cli.Command {
	return cli.Command{
		Name:   "list-machines",
		Usage:  "Enumerate the current hosts in the cluster",
		Action: listMachinesAction,
		Flags: []cli.Flag{
			cli.BoolFlag{"full, l", "Do not ellipsize fields on output"},
			cli.BoolFlag{"no-legend", "Do not print a legend (column headers)"},
		},
	}
}

func listMachinesAction(c *cli.Context) {
	r := getRegistry(c)

	if !c.Bool("no-legend") {
		fmt.Fprintln(out, "MACHINE\tIP\tMETADATA")
	}

	full := c.Bool("full")

	for _, m := range r.GetActiveMachines() {
		mach := m.BootId
		if !full {
			mach = ellipsize(mach, 8)
		}

		fmt.Fprintf(out, "%s\t%s\t%s\n", mach, m.PublicIP, formatMetadata(m.Metadata))
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
