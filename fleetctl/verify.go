package main

import (
	"fmt"
	"os"
)

var cmdVerifyUnit = &Command{
	Name:    "verify",
	Summary: "DEPRECATED - Verify unit file signatures using local SSH identities",
	Usage:   "UNIT",
	Description: `This command is deprecated - it is being removed from fleetctl.
	
Outputs whether or not unit file fits its signature. Useful to secure
the data of a unit.`,
	Run: runVerifyUnit,
}

func runVerifyUnit(args []string) (exit int) {
	fmt.Fprintln(os.Stderr, "WARNING: The signed/verified units feature is DEPRECATED and should not be used. It will be completely removed from fleet and fleetctl.")

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		return 1
	}

	name := unitNameMangle(args[0])
	u, err := cAPI.Unit(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving Unit %s: %v", name, err)
		return 1
	}
	if u == nil {
		fmt.Fprintf(os.Stderr, "Unit %s not found.\n", name)
		return 1
	}

	err = verifyUnit(u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	fmt.Printf("Succeeded verifying unit signature for Unit %s.\n", u.Name)
	return
}
