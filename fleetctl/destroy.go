package main

import (
	"fmt"
	"path"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
)

func newDestroyUnitCommand() cli.Command {
	return cli.Command{
		Name:  "destroy",
		Usage: "Destroy one or more units in the cluster",
		Description: `Completely remove a running or submitted unit from the cluster.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Destroyed units are impossible to start unless re-submitted.`,
		Action: destroyUnitsAction,
	}
}

func destroyUnitsAction(c *cli.Context) {
	for _, v := range c.Args() {
		name := path.Base(v)
		registryCtl.StopJob(name)
		registryCtl.DestroyJob(name)
		fmt.Printf("Destroyed Job %s\n", name)
	}
}
