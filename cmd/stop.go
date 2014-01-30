package main

import (
	"path"

	"github.com/codegangsta/cli"
)

func newStopUnitCommand() cli.Command {
	return cli.Command{
		Name:        "stop",
		Usage:       "Halt one or more units in the cluster",
		Action:      stopUnitAction,
	}
}

func stopUnitAction(c *cli.Context) {
	r := getRegistry(c)

	for _, v := range c.Args() {
		name := path.Base(v)
		r.StopJob(name)
	}
}
