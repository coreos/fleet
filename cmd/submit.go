package main

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func newSubmitUnitCommand() cli.Command {
	return cli.Command{
		Name: "submit",
		Usage:  "Upload one or more units to the cluster",
		Action: submitUnitsAction,
	}
}

func submitUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	payloads := make([]job.JobPayload, len(c.Args()))
	for i, v := range c.Args() {
		out, err := ioutil.ReadFile(v)
		if err != nil {
			fmt.Printf("%s: No such file or directory\n", v)
			return
		}

		unitFile := unit.NewSystemdUnitFile(string(out))

		name := path.Base(v)
		payload := job.NewJobPayload(name, *unitFile)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		payloads[i] = *payload
	}

	req, err := job.NewJobRequest(payloads)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	r.AddRequest(req)
}
