package main

import (
	"fmt"
	"os"
	"path"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/sign"
)

var cmdSubmitUnit = &Command{
	Name:    "submit",
	Summary: "Upload one or more units to the cluster without starting them",
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
	cmdSubmitUnit.Flags.BoolVar(&sharedFlags.Sign, "sign", false, "Sign unit file signatures and verify submitted units using local SSH identities")
}

func runSubmitUnits(args []string) (exit int) {
	_, err := findOrCreateJobs(args, sharedFlags.sign)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed creating jobs: %v", err)
		return 1
	}
	return 0
}

// findOrCreateJobs queries the Registry for Jobs matching the given names.
// If the Jobs are not found in the Registry, it submits them as new Jobs.
// signPayloads controls whether signatures are created and/or checked.
// It returns a slice of Jobs and any error encountered.
func findOrCreateJobs(names []string, signPayloads bool) ([]job.Job, error) {
	var err error

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

	jobs := make([]job.Job, len(names))
	for i, v := range names {
		name := path.Base(v)
		j := registryCtl.GetJob(name)
		if j == nil {
			log.V(1).Infof("Job(%s) not found in Registry", name)
			payload, err := getJobPayloadFromFile(v)
			if err != nil {
				return nil, fmt.Errorf("Failed getting Payload(%s) from file: %v", name, err)
			}

			log.V(1).Infof("Payload(%s) found in local filesystem", name)
			j = job.NewJob(name, *payload)

			err = registryCtl.CreateJob(j)
			if err != nil {
				return nil, fmt.Errorf("Failed creating job %s: %v", j.Name, err)
			}

			log.V(1).Infof("Created Job(%s) in Registry", j.Name)

			if signPayloads {
				s, err := sc.SignPayload(payload)
				if err != nil {
					return nil, fmt.Errorf("Failed creating signature for Payload(%s): %v", payload.Name, err)
				}

				registryCtl.CreateSignatureSet(s)
				log.V(1).Infof("Created signature for Payload(%s)", name)
			}
		} else {
			log.V(1).Infof("Found Job(%s) in Registry", name)
		}

		if signPayloads {
			s := registryCtl.GetSignatureSetOfPayload(name)
			ok, err := sv.VerifyPayload(&(j.Payload), s)
			if !ok || err != nil {
				return nil, fmt.Errorf("Failed checking signature for Payload(%s): %v", j.payload.Name, err)
			}

			log.V(1).Infof("Verified signature of Payload(%s)", j.Payload.Name)
		}

		jobs[i] = *j
	}

	return jobs, nil
}
