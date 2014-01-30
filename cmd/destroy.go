package main

import (
	"path"

	"github.com/codegangsta/cli"
)

func newDestroyUnitCommand() cli.Command {
	return cli.Command{
		Name:        "destroy",
		Usage:       "Destroy one or more units",
		Description: `Remove one or more units from the cluster`,
		Action:      destroyUnitsAction,
	}
}

func destroyUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	for _, v := range c.Args() {
		name := path.Base(v)
		r.StopJob(name)
		r.DestroyPayload(name)
	}
}
