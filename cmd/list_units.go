package main

import (
	"fmt"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/registry"
)

func listUnits(c *cli.Context) {
	r := registry.New()

	fmt.Fprintln(out, "UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")

	for _, j := range r.GetGlobalJobs() {
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
