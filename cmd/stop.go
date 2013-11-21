package main

import (
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/registry"
)

func stopUnit(c *cli.Context) {
	r := registry.New()

	for _, v := range c.Args() {
		baseName := path.Base(v)
		j, _ := job.NewJob(baseName, nil, nil)
		r.StopJob(j)
	}
}
