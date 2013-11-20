package main

import (
	"os"
	"github.com/codegangsta/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "corectl"
	app.Usage = "corectl is a command line driven interface to the cluster wide CoreOS init system."
	app.Action = listUnits

	app.Commands = []cli.Command{
		{
			Name:      "list-units",
			Usage:     "List installed unit files",
			Action: listUnits,
		},
	}

	app.Run(os.Args)
}
