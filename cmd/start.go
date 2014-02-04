package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
)

func newStartUnitCommand() cli.Command {
	return cli.Command{
		Name: "start",
		Flags: []cli.Flag{
			cli.StringFlag{"require", "", "Filter suitable hosts with a set of requirements. Format is comma-delimited list of <key>=<value> pairs."},
		},
		Usage:  "Schedule and execute one or more units already loaded in the cluster",
		Action: startUnitAction,
	}
}

func startUnitAction(c *cli.Context) {
	r := getRegistry(c)

	payloads := make([]job.JobPayload, len(c.Args()))
	for i, v := range c.Args() {
		name := path.Base(v)
		payload := r.GetPayload(name)
		if payload == nil {
			fmt.Printf("Could not find payload %s\n", name)
			return
		}

		payloads[i] = *payload
	}

	requirements := parseRequirements(c.String("require"))

	// TODO: This must be done in a transaction!
	for _, jp := range payloads {
		j := job.NewJob(jp.Name, requirements, &jp, nil)
		r.CreateJob(j)
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
