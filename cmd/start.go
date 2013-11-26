package main

import (
	"io/ioutil"
	"path"

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

		name := path.Base(v)
		payload := job.JobPayload{name, string(out)}
		if err != nil {
			logger.Print(err)
		} else {
			j, _ := job.NewJob(name, nil, &payload)
			r.StartJob(j)
		}
	}
}
