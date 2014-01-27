package main

import (
	"os"
	"text/tabwriter"

	"github.com/codegangsta/cli"
	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/coreinit/registry"
)

var out *tabwriter.Writer

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
}

func getRegistry(context *cli.Context) *registry.Registry {
	endpoint := context.GlobalString("endpoint")
	client := etcd.NewClient([]string{endpoint})
	return registry.New(client)
}

func main() {
	app := cli.NewApp()
	app.Name = "corectl"
	app.Usage = "corectl is a command line driven interface to the cluster wide CoreOS init system."

	app.Flags = []cli.Flag{
		cli.StringFlag{"endpoint", "http://127.0.0.1:4001", "Coreinit Engine API endpoint (etcd)"},
	}

	app.Commands = []cli.Command{
		newListUnitsCommand(),
		newStartUnitCommand(),
		newStopUnitCommand(),
		newStatusUnitsCommand(),
		newCatUnitCommand(),
		newListMachinesCommand(),
		newJournalCommand(),
	}

	app.Run(os.Args)
}
