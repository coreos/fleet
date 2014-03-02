package main

import (
	"path"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
)

func newStopUnitCommand() cli.Command {
	return cli.Command{
		Name:	"stop",
		Usage:	"Halt one or more units in the cluster",
		Description: `Stop one or more units from running in the cluster, but allow them to be
started again in the future.

Instructs systemd on the host machine to stop the unit, deferring to systemd
completely for any custom stop directives (i.e. ExecStop option in the unit
file).

Stop a single unit:
fleetctl stop foo.service

Stop an entire directory of units with glob matching:
fleetctl stop myservice/*`,
		Action:	stopUnitAction,
	}
}

func stopUnitAction(c *cli.Context) {
	r := getRegistry()

	for _, v := range c.Args() {
		name := path.Base(v)
		r.StopJob(name)
	}
}
