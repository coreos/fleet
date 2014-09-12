package main

var cmdSubmitUnit = &Command{
	Name:    "submit",
	Summary: "Upload one or more units to the cluster without starting them",
	Usage:   "UNIT...",
	Description: `Upload one or more units to the cluster without starting them. Useful
for validating units before they are started.

This operation is idempotent; if a named unit already exists in the cluster, it will not be resubmitted.

Submit a single unit:
	fleetctl submit foo.service

Submit a directory of units with glob matching:
	fleetctl submit myservice/*`,
	Run: runSubmitUnits,
}

func init() {
	cmdSubmitUnit.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "DEPRECATED - this option cannot be used")
}

func runSubmitUnits(args []string) (exit int) {
	if err := lazyCreateUnits(args); err != nil {
		stderr("Error creating units: %v", err)
		exit = 1
	}
	return
}
