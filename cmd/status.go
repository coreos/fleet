package main

import (
	"fmt"
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/registry"
)

func getUnitsStatus(c *cli.Context) {
	r := registry.New()

	for i, v := range c.Args() {
		// This extra newline here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		name := path.Base(v)
		j, _ := job.NewJob(name, nil, nil)
		j.State = r.GetJobState(j)

		fmt.Printf("%s\n", j.Name)
		fmt.Print("\tLoaded: loaded\n")
		fmt.Printf("\tActive: %s\n", j.State.State)
		for _, sock := range j.State.Sockets {
			fmt.Printf("\tListen: %s\n", sock)
		}
		fmt.Print("\n")
	}
}
