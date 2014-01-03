package main

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
)

func newStartUnitCommand() cli.Command {
	return cli.Command{
		Name: "start",
		Flags: []cli.Flag{
			cli.StringFlag{"require", "", "Filter hosts with a set of requirements. Format is comma-delimited list of <key>=<value> pairs."},
		},
		Usage:  "Start (activate) one or more units",
		Action: startUnitAction,
	}
}

func startUnitAction(c *cli.Context) {
	r := getRegistry(c)

	requirements := parseRequirements(c.String("require"))

	payloads := make([]job.JobPayload, len(c.Args()))
	for i, v := range c.Args() {
		out, err := ioutil.ReadFile(v)
		if err != nil {
			fmt.Printf("%s: No such file or directory\n", v)
			return
		}

		name := path.Base(v)
		payload, err := job.NewJobPayload(name, string(out), requirements)
		if err != nil {
			fmt.Println(err.Error())
			return
		} else {
			payloads[i] = *payload
		}
	}

	req, err := job.NewJobRequest(payloads)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	r.AddRequest(req)
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

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		add(key, val)
	}

	return reqs
}
