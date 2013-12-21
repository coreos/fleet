package main

import (
	"fmt"

	"github.com/codegangsta/cli"
)

func newListMachinesCommand() cli.Command {
	return cli.Command{
		Name:        "list-machines",
		Usage:       "List all machines.",
		Description: "",
		Action:      listMachinesAction,
	}
}

func listMachinesAction(c *cli.Context) {
	r := getRegistry(c)

	fmt.Fprintln(out, "MACHINE")

	for _, m := range r.GetActiveMachines() {
		fmt.Fprintf(out, "%s\n", m.BootId)
	}

	out.Flush()
}
