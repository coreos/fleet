package main

import (
	"fmt"
	"path"
	"syscall"

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
		fmt.Println("One unit file must be provided.")
		syscall.Exit(1)
	}

	name := path.Base(c.Args()[0])
	payload := r.GetPayload(name)

	if payload == nil {
		fmt.Println("Job not found.")
		syscall.Exit(1)
	}

	sv, err := sign.NewSignatureVerifierFromSSHAgent()
	if err != nil {
		fmt.Println("Fail to create SignatureVerifier:", err)
		return
	}

	s := r.GetSignatureSetOfPayload(name)
	ok, err := sv.VerifyPayload(payload, s)
	if !ok || err != nil {
		fmt.Printf("Check of payload %s failed: %v\n", payload.Name, err)
		return
	}

	fmt.Printf("Succeed to verify job(%s).\n", payload.Name)
}
