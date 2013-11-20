package main

import (
	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/registry"
)

func stopUnit(c *cli.Context) {
	r := registry.New()

	for _, v := range c.Args() {
		job := job.NewJob(v, nil, nil)
		r.StopJob(job)
	}
}
