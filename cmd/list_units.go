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

		var activeState string
		var loadState string
		var mach string
		if js != nil {
			activeState = js.State
			loadState = "loaded"
			mach = js.Machine.String()
		} else {
			activeState = "-"
			loadState = "-"
			mach = "-"
		}

		fmt.Fprintf(out, "%s\t%s\t%s\t-\t-\t%s\n", j.Name, loadState, activeState, mach)
	}

	out.Flush()
}
