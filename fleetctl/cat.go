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
		fmt.Println("One unit file must be provided.")
		os.Exit(1)
	}

	name := path.Base(c.Args()[0])
	payload := registryCtl.GetPayload(name)

	if payload == nil {
		fmt.Println("Job not found.")
		os.Exit(1)
	}

	fmt.Print(payload.Unit.String())
}
