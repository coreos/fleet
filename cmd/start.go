package main

import (
	"strings"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/registry"
)

func startUnit(c *cli.Context) {
	r := registry.New()

	args := c.Args()
	exec := args[1:]
	payload := job.NewJobPayload(strings.Join(exec, " "))
	job := job.NewJob(args[0], nil, payload)

	r.StartJob(job)
}
