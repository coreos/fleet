package main

import (
	"fmt"
	"os"
	"path"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

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
	_, err := submitPayloads(c.Args(), c.Bool("sign"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating payloads: %v", err)
		os.Exit(1)
	}
}

func submitPayloads(names []string, signPayloads bool) ([]job.JobPayload, error) {
	var err error

	// If signing is explicitly set to on, verification will be done also.
	var sc *sign.SignatureCreator
	var sv *sign.SignatureVerifier
	if signPayloads {
		sc, err = sign.NewSignatureCreatorFromSSHAgent()
		if err != nil {
			return nil, fmt.Errorf("Failed creating SignatureCreator: %v", err)
		}
		sv, err = sign.NewSignatureVerifierFromSSHAgent()
		if err != nil {
			return nil, fmt.Errorf("Failed creating SignatureVerifier: %v", err)
		}
	}

	payloads := make([]job.JobPayload, len(names))
	for i, v := range names {
		name := path.Base(v)
		payload := registryCtl.GetPayload(name)
		if payload == nil {
			log.V(1).Infof("Payload(%s) not found in Registry", name)
			payload, err = getJobPayloadFromFile(v)
			if err != nil {
				return nil, err
			}

			log.V(1).Infof("Payload(%s) found in local filesystem", name)

			err = registryCtl.CreatePayload(payload)
			if err != nil {
				return nil, fmt.Errorf("Failed creating payload %s: %v", payload.Name, err)
			}

			log.V(1).Infof("Created Payload(%s) in Registry", name)

			if signPayloads {
				s, err := sc.SignPayload(payload)
				if err != nil {
					return nil, fmt.Errorf("Failed creating sign for payload %s: %v", payload.Name, err)
				}

				registryCtl.CreateSignatureSet(s)
				log.V(1).Infof("Signed Payload(%s)", name)
			}
		} else {
			log.V(1).Infof("Found Payload(%s) in Registry", name)
		}

		if signPayloads {
			s := registryCtl.GetSignatureSetOfPayload(name)
			ok, err := sv.VerifyPayload(payload, s)
			if !ok || err != nil {
				return nil, fmt.Errorf("Failed checking payload %s: %v", payload.Name, err)
			}

			log.V(1).Infof("Verified signature of Payload(%s)", name)
		}

		payloads[i] = *payload
	}

	return payloads, nil
}
