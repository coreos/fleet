package main

import (
	"fmt"
	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/registry"
)

func listUnits(c *cli.Context) {
	r := registry.New()

	machines := r.GetActiveMachines()

	println("UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")

	for _, m := range machines {
		for _, j := range r.GetMachineJobs(&m) {
			js := r.GetJobState(&j)
			var state string
			if js != nil {
				state = js.State
			} else {
				state = "-"
			}

			fmt.Printf("%s\tloaded\t%s\t-\t-\t%s\n", j.Name, state, m.String())
		}
	}
}
