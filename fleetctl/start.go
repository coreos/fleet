package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/sign"
)

const (
	unitCheckInterval = 1 * time.Second
)

func newStartUnitCommand() cli.Command {
	return cli.Command{
		Name:  "start",
		Usage: "Schedule and execute one or more units in the cluster",
		Description: `Start one or many units on the cluster. Select units to start by glob matching
for units in the current working directory or matching names of previously
submitted units.

Start a single unit:
fleetctl start foo.service

Start an entire directory of units with glob matching:
fleetctl start myservice/*

You may filter suitable hosts based on metadata provided by the machine.
Machine metadata is located in the fleet configuration file.

Start a unit on any "us-east" machine:
fleetctl start --require region=us-east foo.service`,
		Flags: []cli.Flag{
			cli.StringFlag{"require", "", "Filter suitable hosts with a set of requirements. Format is comma-delimited list of <key>=<value> pairs."},
			cli.BoolFlag{"sign", "Sign unit file signatures using local SSH identities"},
			cli.BoolFlag{"verify", "Verify unit file signatures using local SSH identities"},
			cli.IntFlag{"block-attempts", 10, "Wait until the jobs are scheduled. Perform N attempts before giving up, 10 by default."},
			cli.BoolFlag{"no-block", "Do not wait until the units have been scheduled to exit start."},
		},
		Action: startUnitAction,
	}
}

func startUnitAction(c *cli.Context) {
	var err error

	// If signing is explicitly set to on, verification will be done also.
	toSign := c.Bool("sign")
	var sc *sign.SignatureCreator
	var sv *sign.SignatureVerifier
	if toSign {
		var err error
		sc, err = sign.NewSignatureCreatorFromSSHAgent()
		if err != nil {
			fmt.Println("Fail to create SignatureCreator:", err)
			return
		}
		sv, err = sign.NewSignatureVerifierFromSSHAgent()
		if err != nil {
			fmt.Println("Fail to create SignatureVerifier:", err)
			return
		}
	}

	payloads := make([]job.JobPayload, len(c.Args()))
	for i, v := range c.Args() {
		name := path.Base(v)
		payload := registryCtl.GetPayload(name)
		if payload == nil {
			payload, err = getJobPayloadFromFile(v)
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			err = registryCtl.CreatePayload(payload)
			if err != nil {
				fmt.Printf("Creation of payload %s failed: %v\n", payload.Name, err)
				return
			}
			if toSign {
				s, err := sc.SignPayload(payload)
				if err != nil {
					fmt.Printf("Creation of sign for payload %s failed: %v\n", payload.Name, err)
					return
				}
				registryCtl.CreateSignatureSet(s)
			}
		}
		if toSign {
			s := registryCtl.GetSignatureSetOfPayload(name)
			ok, err := sv.VerifyPayload(payload, s)
			if !ok || err != nil {
				fmt.Printf("Check of payload %s failed: %v\n", payload.Name, err)
				return
			}
		}

		payloads[i] = *payload
	}

	requirements := parseRequirements(c.String("require"))

	// TODO: This must be done in a transaction!
	registeredJobs := make(map[string]bool)
	for _, jp := range payloads {
		j := job.NewJob(jp.Name, requirements, &jp, nil)
		err := registryCtl.CreateJob(j)
		if err != nil {
			fmt.Printf("Creation of job %s failed: %v\n", j.Name, err)
			continue
		}
		registeredJobs[j.Name] = true
	}

	if !c.Bool("no-block") {
		waitForScheduledUnits(registeredJobs, c.Int("block-attempts"), os.Stdout)
	}
}

func parseRequirements(arg string) map[string][]string {
	reqs := make(map[string][]string, 0)

	add := func(key, val string) {
		vals, ok := reqs[key]
		if !ok {
			vals = make([]string, 0)
			reqs[key] = vals
		}
		vals = append(vals, val)
		reqs[key] = vals
	}

	for _, pair := range strings.Split(arg, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := fmt.Sprintf("MachineMetadata%s", strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		add(key, val)
	}

	return reqs
}

func waitForScheduledUnits(jobs map[string]bool, maxAttempts int, out io.Writer) {
	var wg sync.WaitGroup

	for jobName, _ := range jobs {
		wg.Add(1)
		go checkJobTarget(jobName, maxAttempts, out, &wg)
	}

	wg.Wait()
}

func checkJobTarget(jobName string, maxAttempts int, out io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()

	for attempts := 0; attempts < maxAttempts; attempts++ {
		ms := registryCtl.GetJobTarget(jobName)

		if ms != nil {
			m := registryCtl.GetMachineState(ms.BootID)
			fmt.Fprintf(out, "Job %s started on %s\n", jobName, machineFullLegend(*m, false))
			return
		}

		time.Sleep(unitCheckInterval)
	}
	fmt.Fprintf(out, "Job %s in queue to be scheduled\n", jobName)
}
