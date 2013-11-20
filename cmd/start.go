package main

import (
	"io/ioutil"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/registry"
)

func startUnit(c *cli.Context) {
	r := registry.New()

	for _, v := range c.Args() {
		out, err := ioutil.ReadFile(v)
		if err != nil {
			logger.Fatalf("%s: No such file or directory\n", v)
		}
		payload := job.NewJobPayload(string(out))
		job := job.NewJob(v, nil, payload)
		r.StartJob(job)
	}
}
