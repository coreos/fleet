package main

import (
	"fmt"
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
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
		j, _ := job.NewJob(name, nil, nil)
		j.State = r.GetJobState(j)

		loadState := "-"
		activeState := "-"
		subState := "-"

		if j.State != nil {
			loadState = j.State.LoadState
			activeState = j.State.ActiveState
			subState = j.State.SubState
		}

		fmt.Printf("%s\n", j.Name)
		fmt.Printf("\tLoaded: %s\n", loadState)
		fmt.Printf("\tActive: %s (%s)\n", activeState, subState)
		if j.State != nil {
			for _, sock := range j.State.Sockets {
				fmt.Printf("\tListen: %s\n", sock)
			}
		}
		fmt.Print("\n")
	}
}
