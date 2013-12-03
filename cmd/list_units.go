package main

import (
	"fmt"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/registry"
)

func newListUnitsCommand() cli.Command {
	return cli.Command{
		Name:  "list-units",
		Usage: "List installed unit files",
		Description: `List all of the units that are scheduled on the
	cluster and their current state.`,
		Action: listUnitsAction,
	}
}

func listUnitsAction(c *cli.Context) {
	r := registry.New()

	fmt.Fprintln(out, "UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")

	for _, j := range r.GetScheduledJobs() {
		js := r.GetJobState(&j)

		loadState := "-"
		activeState := "-"
		subState := "-"
		mach := "-"

		if js != nil {
			loadState = js.LoadState
			activeState = js.ActiveState
			subState = js.SubState
			mach = js.Machine.String()
		}

		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t-\t%s\n", j.Name, loadState, activeState, subState, mach)
	}

	out.Flush()
}
