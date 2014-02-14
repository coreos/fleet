package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
)

func newStartUnitCommand() cli.Command {
	return cli.Command{
		Name:	"start",
		Usage:	"Schedule and execute one or more units in the cluster",
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
fleetctl start --require region,us-east foo.service`,
		Flags: []cli.Flag{
			cli.StringFlag{"require", "", "Filter suitable hosts with a set of requirements. Format is comma-delimited list of <key>=<value> pairs."},
		},
		Action:	startUnitAction,
	}
}

func startUnitAction(c *cli.Context) {
	var err error
	r := getRegistry(c)

	payloads := make([]job.JobPayload, len(c.Args()))
	for i, v := range c.Args() {
		name := path.Base(v)
		payload := r.GetPayload(name)
		if payload == nil {
			payload, err = getJobPayloadFromFile(v)
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			err = r.CreatePayload(payload)
			if err != nil {
				fmt.Printf("Creation of payload %s failed: %v\n", payload.Name, err)
				return
			}
		}

		payloads[i] = *payload
	}

	requirements := parseRequirements(c.String("require"))

	// TODO: This must be done in a transaction!
	for _, jp := range payloads {
		j := job.NewJob(jp.Name, requirements, &jp, nil)
		err := r.CreateJob(j)
		if err != nil {
			fmt.Printf("Creation of job %s failed: %v\n", j.Name, err)
		}
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
