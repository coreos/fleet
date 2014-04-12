package main

import (
	"fmt"
	"os"
	"path"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/sign"
)

func newVerifyUnitCommand() cli.Command {
	return cli.Command{
		Name:  "verify",
		Usage: "Verify unit file signatures using local SSH identities",
		Description: `Outputs whether or not unit file fits its signature. Useful to secure
the data of a unit.`,
		Action: verifyUnitAction,
	}
}

func verifyUnitAction(c *cli.Context) {
	r := getRegistry()

	if len(c.Args()) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		os.Exit(1)
	}

	name := path.Base(c.Args()[0])
	j := r.GetJob(name)
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s not found.\n", name)
		os.Exit(1)
	}

	sv, err := sign.NewSignatureVerifierFromSSHAgent()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating SignatureVerifier: %v\n", err)
		os.Exit(1)
	}

	s := r.GetSignatureSetOfPayload(name)
	ok, err := sv.VerifyPayload(&(j.Payload), s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed checking payload %s: %v\n", j.Payload.Name, err)
		os.Exit(1)
	}

	if !ok {
		fmt.Printf("Failed to verify job(%s).\n", j.Payload.Name)
		os.Exit(1)
	}
	fmt.Printf("Succeed to verify job(%s).\n", j.Payload.Name)
}
