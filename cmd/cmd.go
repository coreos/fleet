package main

import (
	"log"
	"os"

	"github.com/codegangsta/cli"
)

var logger *log.Logger

func main() {
	logger = log.New(os.Stderr, "", 0)
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
