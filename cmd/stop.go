package main

import (
	"path"

	"github.com/codegangsta/cli"
)

func newStopUnitCommand() cli.Command {
	return cli.Command{
		Name:        "stop",
		Usage:       "Stop one or more units",
		Description: `Remove one or more jobs from the cluster schedule.`,
		Action:      stopUnitAction,
	}
}

func stopUnitAction(c *cli.Context) {
	r := getRegistry(c)

	for _, v := range c.Args() {
		name := path.Base(v)
		for _, j := range r.FindJobs(name) {
			r.CancelJob(j.Name)
			r.DestroyJob(j.Name)
		}
	}
}
