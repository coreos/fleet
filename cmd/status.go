package main

import (
	"fmt"
	"path"

	"github.com/codegangsta/cli"
)

func newStatusUnitsCommand() cli.Command {
	return cli.Command{
		Name:        "status",
		Usage:       "Fetch the status of one or more units",
		Description: ``,
		Action:      statusUnitsAction,
	}
}

func statusUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	for i, v := range c.Args() {
		// This extra newline here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		name := path.Base(v)
		for _, j := range r.FindJobs(name) {
			state := r.GetJobState(&j)

			loadState := "-"
			activeState := "-"
			subState := "-"

			if state != nil {
				loadState = state.LoadState
				activeState = state.ActiveState
				subState = state.SubState
			}

			fmt.Printf("%s\n", j.Name)
			fmt.Printf("\tLoaded: %s\n", loadState)
			fmt.Printf("\tActive: %s (%s)\n", activeState, subState)
			if state != nil {
				for _, sock := range state.Sockets {
					fmt.Printf("\tListen: %s\n", sock)
				}
			}
			fmt.Print("\n")
		}
	}
}
