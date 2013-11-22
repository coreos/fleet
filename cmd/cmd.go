package main

import (
	"log"
	"os"
	"text/tabwriter"

	"github.com/codegangsta/cli"
)

var logger *log.Logger
var out *tabwriter.Writer

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
}

func main() {
	logger = log.New(os.Stderr, "", 0)

	app := cli.NewApp()
	app.Name = "corectl"
	app.Usage = "corectl is a command line driven interface to the cluster wide CoreOS init system."
	app.Action = listUnits

	app.Commands = []cli.Command{
		{
			Name:  "list-units",
			Usage: "List installed unit files",
			Description: `List all of the units that are scheduled on the
cluster and their current state.`,
			Action: listUnits,
		},
		{
			Name:  "start",
			Usage: "Start (activate) one or more units",
			Description: `Start adds one or more units to the cluster schedule.
Once scheduled the cluster will ensure that the unit is
running on one machine.`,
			Action: startUnit,
		},
		{
			Name:        "stop",
			Usage:       "Stop one or more units",
			Description: `Remove one or more jobs from the cluster schedule.`,
			Action:      stopUnit,
		},
	}

	app.Run(os.Args)
}
