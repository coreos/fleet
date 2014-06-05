package main

import (
	"fmt"
	"os"
)

var (
	cmdCatUnit = &Command{
		Name:    "cat",
		Summary: "Output the contents of a submitted unit",
		Usage:   "UNIT",
		Description: `Outputs the unit file that is currently loaded in the cluster. Useful to verify
the correct version of a unit is running.`,
		Run: runCatUnit,
	}
)

func runCatUnit(args []string) (exit int) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		return 1
	}

	name := unitNameMangle(args[0])
	j, err := registryCtl.GetJob(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving Job %s: %v\n", name, err)
		return 1
	}
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s not found.\n", name)
		return 1
	}

	fmt.Print(j.Unit.String())
	return
}
