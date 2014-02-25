package main

import (
	"fmt"
	"path"
	"syscall"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/sign"
)

func newCatUnitCommand() cli.Command {
	return cli.Command{
		Name:	"cat",
		Usage:	"Output the contents of a submitted unit",
		Description: `Outputs the unit file that is currently loaded in the cluster. Useful to verify
the correct version of a unit is running.`,
		Flags:	[]cli.Flag{
			cli.BoolFlag{"verify", "Verify unit file signatures using local SSH identities"},
		},
		Action:	printUnitAction,
	}
}

func printUnitAction(c *cli.Context) {
	r := getRegistry(c)

	toVerify := c.Bool("verify")
	var sv *sign.SignatureVerifier
	if toVerify {
		var err error
		sv, err = sign.NewSignatureVerifierFromSSHAgent()
		if err != nil {
			fmt.Println("Fail to create SignatureVerifier:", err)
			return
		}
	}

	if len(c.Args()) != 1 {
		fmt.Println("One unit file must be provided.")
		syscall.Exit(1)
	}

	name := path.Base(c.Args()[0])
	payload := r.GetPayload(name)

	if payload == nil {
		fmt.Println("Job not found.")
		syscall.Exit(1)
	}

	if toVerify {
		s := r.GetSignatureSetOfPayload(name)
		ok, err := sv.VerifyPayload(payload, s)
		if !ok || err != nil {
			fmt.Printf("Check of payload %s failed: %v\n", payload.Name, err)
			return
		}
	}

	fmt.Print(payload.Unit.String())
}
