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
		Flags: []cli.Flag{
			cli.BoolFlag{"all-machines", "Run units on all machines."},
			cli.IntFlag{"count", 1, "Run N instances of these units."},
		},
		Usage: "Start (activate) one or more units",
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

	//TODO: Handle error response from NewJobRequest
	req, _ := job.NewJobRequest(payloads, nil)

	if c.Bool("all-machines") {
		req.SetFlag(job.RequestAllMachines)
		req.Count = 1
	} else {
		req.Count = c.Int("count")
	}

	r.AddRequest(req)
}
