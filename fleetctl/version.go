package main

import (
	"github.com/coreos/fleet/version"
)

var cmdVersion = &Command{
	Name:        "version",
	Description: "Print the version and exit",
	Summary:     "Print the version and exit",
	Run:         runVersion,
}

func runVersion(args []string) (exit int) {
	stdout("fleetctl version %s", version.Version)
	return
}
