package main

import (
	"fmt"
	"os"
	"path"

	"github.com/coreos/fleet/sign"
)

var cmdVerifyUnit = &Command{
	Name:    "verify",
	Summary: "Verify unit file signatures using local SSH identities",
	Usage:   "UNIT",
	Description: `Outputs whether or not unit file fits its signature. Useful to secure
the data of a unit.`,
	Run: runVerifyUnit,
}

func runVerifyUnit(args []string) (exit int) {

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		return 1
	}

	name := path.Base(args[0])
	j := registryCtl.GetJob(name)
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s not found.\n", name)
		return 1
	}

	sv, err := sign.NewSignatureVerifierFromSSHAgent()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating SignatureVerifier: %v\n", err)
		return 1
	}

	s := registryCtl.GetSignatureSetOfPayload(name)
	ok, err := sv.VerifyPayload(&(j.Payload), s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed checking payload %s: %v\n", j.Payload.Name, err)
		return 1
	}

	if !ok {
		fmt.Printf("Failed to verify job(%s).\n", j.Payload.Name)
		return 1
	}
	fmt.Printf("Succeeded verifying job(%s).\n", j.Payload.Name)
	return
}
