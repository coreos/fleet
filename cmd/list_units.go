package main

import (
	"fmt"

	"github.com/codegangsta/cli"
)

func newListUnitsCommand() cli.Command {
	return cli.Command{
		Name:   "list-units",
		Usage:  "Enumerate units loaded in the cluster",
		Action: listUnitsAction,
		Flags: []cli.Flag{
			cli.BoolFlag{"full, l", "Do not ellipsize fields on output"},
			cli.BoolFlag{"no-legend", "Do not print a legend (column headers)"},
		},
	}
}

func listUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	if !c.Bool("no-legend") {
		fmt.Fprintln(out, "UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")
	}

	full := c.Bool("full")

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
				if !full {
					mach = ellipsize(mach, 8)
				}
			}
		}

		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t-\t%s\n", jp.Name, loadState, activeState, subState, mach)
	}

	out.Flush()
}
