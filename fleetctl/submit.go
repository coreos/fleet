package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/sign"
)

func newSubmitUnitCommand() cli.Command {
	return cli.Command{
		Name:  "submit",
		Usage: "Upload one or more units to the cluster without starting them",
		Description: `Upload one or more units to the cluster without starting them. Useful
validating units before they are started.

Submit a single unit:
fleetctl submit foo.service

Submit a directory of units with glob matching:
fleetctl submit myservice/*`,
		Flags: []cli.Flag{
			cli.BoolFlag{"sign", "Sign unit file signatures using local SSH identities"},
		},
		Action: submitUnitsAction,
	}
}

func submitUnitsAction(c *cli.Context) {
	toSign := c.Bool("sign")
	var sc *sign.SignatureCreator
	if toSign {
		var err error
		sc, err = sign.NewSignatureCreatorFromSSHAgent()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed creating SignatureVerifier: %v\n", err)
			os.Exit(1)
		}
	}

	// First, validate each of the provided payloads
	payloads := make([]job.JobPayload, len(c.Args()))
	for i, v := range c.Args() {
		payload, err := getJobPayloadFromFile(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed fetching payload from %v: %v\n", v, err)
			os.Exit(1)
		}
		payloads[i] = *payload
	}

	// Only after all the provided payloads have been validated
	// do we push any changes to the Registry
	for _, payload := range payloads {
		err := registryCtl.CreatePayload(&payload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed creating payload %s: %v\n", payload.Name, err)
			os.Exit(1)
		}
		if toSign {
			s, err := sc.SignPayload(&payload)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed creating sign for payload %s: %v\n", payload.Name, err)
				os.Exit(1)
			}
			registryCtl.CreateSignatureSet(s)
		}
	}
}
