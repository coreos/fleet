package main

import (
	"fmt"
	"os"
)

var cmdSubmitUnit = &Command{
	Name:    "submit",
	Summary: "Upload one or more units to the cluster without starting them",
	Usage:   "[--sign] UNIT...",
	Description: `Upload one or more units to the cluster without starting them. Useful
for validating units before they are started.

This operation is idempotent; if a named unit already exists in the cluster, it will not be resubmitted.
However, its signature will still be validated if "sign" is enabled.

Submit a single unit:
	fleetctl submit foo.service

Submit a directory of units with glob matching:
	fleetctl submit myservice/*`,
	Run: runSubmitUnits,
}

func init() {
	cmdSubmitUnit.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "Sign unit files units using local SSH identities")
}

func runSubmitUnits(args []string) (exit int) {
	if err := lazyCreateJobs(args, sharedFlags.Sign); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating jobs: %v\n", err)
		exit = 1
	}
	return
}
