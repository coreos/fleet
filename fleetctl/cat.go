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
	u, err := cAPI.Unit(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving Unit %s: %v\n", name, err)
		return 1
	}
	if u == nil {
		fmt.Fprintf(os.Stderr, "Unit %s not found.\n", name)
		return 1
	}

	fmt.Print(u.Unit.String())
	return
}
