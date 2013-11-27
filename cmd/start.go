package main

import (
	"io/ioutil"
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/registry"
)

func newStartUnitCommand() cli.Command {
	return cli.Command{
		Name:  "start",
		Usage: "Start (activate) one or more units",
		Description: `Start adds one or more units to the cluster schedule.
Once scheduled the cluster will ensure that the unit is
running on one machine.`,
		Action: startUnitAction,
	}
}

func startUnitAction(c *cli.Context) {
	r := registry.New()

	payloads := make([]job.JobPayload, len(c.Args()))

	for i, v := range c.Args() {
		out, err := ioutil.ReadFile(v)
		if err != nil {
			logger.Fatalf("%s: No such file or directory\n", v)
		}

		name := path.Base(v)
		payload := job.JobPayload{name, string(out)}
		if err != nil {
			logger.Fatal(err)
		} else {
			payloads[i] = payload
		}
	}

	for _, p := range payloads {
		println(p.Name)
		println(p.Value)
	}

	//TODO: Handle error response from NewJobRequest
	req, _ := job.NewJobRequest(payloads, nil)
	r.AddRequest(req)
}
