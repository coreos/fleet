package main

import (
	"github.com/codegangsta/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "corectl"
	app.Usage = "corectl is a command line driven interface to the cluster wide CoreOS init system."
	app.Action = listUnits

	app.Commands = []cli.Command{
		{
			Name:   "list-units",
			Usage:  "List installed unit files",
			Action: listUnits,
		},
		{
			Name:   "start",
			Usage:  "Start (activate) one or more units",
			Action: startUnit,
		},
	}

	app.Run(os.Args)
}
