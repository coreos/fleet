package main

import (
	"fmt"
	"strings"

	"github.com/codegangsta/cli"
)

func newListMachinesCommand() cli.Command {
	return cli.Command{
		Name:        "list-machines",
		Usage:       "Enumerate the current hosts in the cluster",
		Action:      listMachinesAction,
	}
}

func listMachinesAction(c *cli.Context) {
	r := getRegistry(c)

	fmt.Fprintln(out, "MACHINE\tIP\tMETADATA")

	for _, m := range r.GetActiveMachines() {
		fmt.Fprintf(out, "%s\t%s\t%s\n", m.BootId, m.PublicIP, formatMetadata(m.Metadata))
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
