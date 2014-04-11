package main

import (
	"fmt"
	"os"
	"path"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
)

func newCatUnitCommand() cli.Command {
	return cli.Command{
		Name:  "cat",
		Usage: "Output the contents of a submitted unit",
		Description: `Outputs the unit file that is currently loaded in the cluster. Useful to verify
the correct version of a unit is running.`,
		Action: printUnitAction,
	}
}

func printUnitAction(c *cli.Context) {
	if len(c.Args()) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		os.Exit(1)
	}

	name := path.Base(c.Args()[0])
	j := registryCtl.GetJob(name)
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s not found.\n", name)
		os.Exit(1)
	}

	fmt.Print(j.Payload.Unit.String())
}
