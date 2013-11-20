package main

import (
	"bytes"
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
		nameBytes := []byte(name)

		var payloadType string
		if bytes.HasSuffix(nameBytes, []byte(".service")) {
			payloadType = "systemd-service"
		} else if bytes.HasSuffix(nameBytes, []byte(".socket")) {
			payloadType = "systemd-socket"
		} else {
			// Unable to handle this job type
			logger.Fatalf("Unrecognized systemd unit: %s\n", v)
			continue
		}

		payload := job.NewJobPayload(payloadType, string(out))
		job := job.NewJob(name, nil, payload)
		r.StartJob(job)
	}
}
