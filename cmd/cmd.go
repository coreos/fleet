package main

import (
	"os"
	"text/tabwriter"

	"github.com/codegangsta/cli"
)

var out *tabwriter.Writer

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
}

func main() {
	app := cli.NewApp()
	app.Name = "corectl"
	app.Usage = "corectl is a command line driven interface to the cluster wide CoreOS init system."

	app.Commands = []cli.Command{
		newListUnitsCommand(),
		newStartUnitCommand(),
		newStopUnitCommand(),
		newStatusUnitsCommand(),
		newCatUnitCommand(),
	}

	app.Run(os.Args)
}
