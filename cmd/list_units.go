package main

import (
	"fmt"

	"github.com/codegangsta/cli"
)

func newListUnitsCommand() cli.Command {
	return cli.Command{
		Name:  "list-units",
		Usage: "Enumerate units loaded in the cluster",
		Action: listUnitsAction,
	}
}

func listUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	fmt.Fprintln(out, "UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")

	for _, jp := range r.GetAllPayloads() {
		js := r.GetJobState(jp.Name)

		loadState := "-"
		activeState := "-"
		subState := "-"
		mach := "-"

		if js != nil {
			loadState = js.LoadState
			activeState = js.ActiveState
			subState = js.SubState
			if js.Machine != nil {
				mach = js.Machine.String()
			}
		}

		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t-\t%s\n", jp.Name, loadState, activeState, subState, mach)
	}

	out.Flush()
}
