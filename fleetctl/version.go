package main

import (
	"fmt"

	"github.com/coreos/fleet/version"
)

var cmdVersion = &Command{
	Name:        "version",
	Description: "Print the version and exit",
	Summary:     "Print the version and exit",
	Run:         runVersion,
}

func runVersion(args []string) (exit int) {
	fmt.Println("fleetctl version", version.Version)
	return
}
