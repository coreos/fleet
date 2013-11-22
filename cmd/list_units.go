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

		var state string
		var mach string
		if js != nil {
			state = js.State
			mach = js.Machine.String()
		} else {
			state = "-"
			mach = "-"
		}

		fmt.Fprintf(out, "%s\tloaded\t%s\t-\t-\t%s\n", j.Name, state, mach)
	}

	out.Flush()
}
