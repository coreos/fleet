package main

import (
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/registry"
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
	r := registry.New()

	for _, v := range c.Args() {
		baseName := path.Base(v)
		j, _ := job.NewJob(baseName, nil, nil)
		r.StopJob(j)
	}
}
