package main

import (
	"fmt"
	"path"
	"syscall"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/registry"
)

func newCatUnitCommand() cli.Command {
	return cli.Command{
		Name:        "cat",
		Usage:       "Print the contents of a unit file to stdout.",
		Description: ``,
		Action:      printUnitAction,
	}
}

func printUnitAction(c *cli.Context) {
	r := registry.New()

	if len(c.Args()) != 1 {
		fmt.Println("One unit file must be provided.")
		syscall.Exit(1)
	}

	name := path.Base(c.Args()[0])

	for jobName, j := range r.GetAllScheduledJobs() {
		if jobName == name {
			fmt.Println(j.Payload.Value)
			return
		}
	}

	fmt.Println("Unit not found.")
	syscall.Exit(1)
}
