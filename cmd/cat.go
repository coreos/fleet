package main

import (
	"fmt"
	"path"
	"syscall"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
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

	j, err := job.NewJob(name, nil, nil)
	if err != nil {
		fmt.Println(err)
		syscall.Exit(1)
	}

	j.Payload = r.GetJobPayload(j)

	if j.Payload == nil {
		fmt.Println("Unit not found.")
		syscall.Exit(1)
	} else {
		fmt.Println(j.Payload.Value)
	}
}
